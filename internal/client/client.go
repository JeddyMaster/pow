package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"pow/internal/pow"
	"pow/pkg/protocol"
)

// Config holds client configuration
type Config struct {
	ServerHost     string
	ServerPort     string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	SolveTimeout   time.Duration
}

// Client represents the TCP client
type Client struct {
	config     Config
	powService pow.SolverService // Client only needs solver operations
	logger     *slog.Logger
}

// NewClient creates a new TCP client instance
func NewClient(config Config, powService pow.SolverService, logger *slog.Logger) *Client {
	return &Client{
		config:     config,
		powService: powService,
		logger:     logger,
	}
}

// RequestQuote connects to the server, solves PoW challenge, and retrieves a quote
func (c *Client) RequestQuote(ctx context.Context) (string, error) {
	addr := net.JoinHostPort(c.config.ServerHost, c.config.ServerPort)
	c.logger.Info("Connecting to server", "address", addr)

	// Connect to server with timeout
	conn, err := net.DialTimeout("tcp", addr, c.config.ConnectTimeout)
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	c.logger.Info("Connected to server")

	// Read challenge from server
	var challengeMsg protocol.ChallengeMessage
	if err := protocol.ReadMessage(conn, &challengeMsg, c.config.ReadTimeout); err != nil {
		return "", fmt.Errorf("failed to read challenge: %w", err)
	}

	c.logger.Info("Challenge received",
		"challenge", challengeMsg.Challenge,
		"difficulty", challengeMsg.Difficulty)

	// Solve PoW challenge
	solveCtx, cancel := context.WithTimeout(ctx, c.config.SolveTimeout)
	defer cancel()

	c.logger.Info("Solving PoW challenge...", "difficulty", challengeMsg.Difficulty)
	startTime := time.Now()

	nonce, err := c.powService.SolveChallenge(solveCtx, challengeMsg.Challenge, challengeMsg.Difficulty)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.logger.Warn("PoW solving timeout",
				"difficulty", challengeMsg.Difficulty,
				"timeout", c.config.SolveTimeout,
				"elapsed", time.Since(startTime))
		} else if errors.Is(err, context.Canceled) {
			c.logger.Info("PoW solving canceled")
		} else {
			c.logger.Error("PoW solving failed", "error", err)
		}
		return "", fmt.Errorf("failed to solve challenge: %w", err)
	}

	solveDuration := time.Since(startTime)
	c.logger.Info("PoW challenge solved",
		"nonce", nonce,
		"duration", solveDuration)

	// Send proof to server
	proofMsg := protocol.ProofMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeProof},
		Challenge:   challengeMsg.Challenge,
		Nonce:       nonce,
	}

	if err := protocol.WriteMessage(conn, proofMsg, c.config.WriteTimeout); err != nil {
		return "", fmt.Errorf("failed to send proof: %w", err)
	}

	c.logger.Info("Proof sent to server")

	// Read response from server (quote or error)
	// Read into json.RawMessage to allow re-parsing
	var rawResponse json.RawMessage
	if err := protocol.ReadMessage(conn, &rawResponse, c.config.ReadTimeout); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse base message to determine type
	var baseMsg protocol.BaseMessage
	if err := json.Unmarshal(rawResponse, &baseMsg); err != nil {
		return "", fmt.Errorf("failed to parse response type: %w", err)
	}

	// Parse into specific message type based on type field
	switch baseMsg.Type {
	case protocol.MsgTypeQuote:
		var quoteMsg protocol.QuoteMessage
		if err := json.Unmarshal(rawResponse, &quoteMsg); err != nil {
			return "", fmt.Errorf("failed to parse quote message: %w", err)
		}
		c.logger.Info("Quote received successfully")
		return quoteMsg.Quote, nil

	case protocol.MsgTypeError:
		var errMsg protocol.ErrorMessage
		if err := json.Unmarshal(rawResponse, &errMsg); err != nil {
			return "", fmt.Errorf("failed to parse error message: %w", err)
		}
		return "", fmt.Errorf("server error: %s", errMsg.Message)

	default:
		return "", fmt.Errorf("unexpected message type: %s", baseMsg.Type)
	}
}
