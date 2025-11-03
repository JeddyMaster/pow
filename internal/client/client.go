package client

import (
	"context"
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
	// First, read into a generic map to check message type
	var response map[string]interface{}
	if err := protocol.ReadMessage(conn, &response, c.config.ReadTimeout); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check message type
	msgType, ok := response["type"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response: missing or invalid 'type' field")
	}

	switch protocol.MessageType(msgType) {
	case protocol.MsgTypeQuote:
		quote, ok := response["quote"].(string)
		if !ok {
			return "", fmt.Errorf("invalid quote response: missing or invalid 'quote' field")
		}
		c.logger.Info("Quote received successfully")
		return quote, nil

	case protocol.MsgTypeError:
		errMsg, ok := response["message"].(string)
		if !ok {
			return "", fmt.Errorf("server returned error without message")
		}
		return "", fmt.Errorf("server error: %s", errMsg)

	default:
		return "", fmt.Errorf("unexpected message type: %s", msgType)
	}
}
