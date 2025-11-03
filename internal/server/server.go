package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"pow/internal/pow"
	"pow/internal/quotes"
	"pow/pkg/protocol"
)

// Config holds server configuration
type Config struct {
	Host            string
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxConnections  int
	ShutdownTimeout time.Duration
}

// Server represents the TCP server
type Server struct {
	config        Config
	powService    pow.Service
	quotesService quotes.Service
	logger        *slog.Logger
	listener      net.Listener
	activeConns   int32
	wg            sync.WaitGroup
	shutdownCh    chan struct{}
	shutdownOnce  sync.Once
}

// NewServer creates a new TCP server instance
func NewServer(config Config, powService pow.Service, quotesService quotes.Service, logger *slog.Logger) *Server {
	return &Server{
		config:        config,
		powService:    powService,
		quotesService: quotesService,
		logger:        logger,
		shutdownCh:    make(chan struct{}),
	}
}

// ListenAndServe starts the server and listens for incoming connections
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := net.JoinHostPort(s.config.Host, s.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener
	s.logger.Info("Server started", "address", addr)

	// Handle graceful shutdown
	go s.handleShutdown(ctx)

	// Accept connections
	for {
		select {
		case <-s.shutdownCh:
			s.logger.Info("Server shutting down...")
			return s.shutdown()
		default:
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.shutdownCh:
					// Listener closed due to shutdown - perform graceful shutdown
					s.logger.Info("Accept failed due to shutdown, cleaning up...")
					return s.shutdown()
				default:
					s.logger.Error("Failed to accept connection", "error", err)
					continue
				}
			}

			// Check max connections limit
			if s.config.MaxConnections > 0 && atomic.LoadInt32(&s.activeConns) >= int32(s.config.MaxConnections) {
				s.logger.Warn("Max connections reached, rejecting connection",
					"remote_addr", conn.RemoteAddr().String())
				conn.Close()
				continue
			}

			// Handle connection in a new goroutine
			s.wg.Add(1)
			atomic.AddInt32(&s.activeConns, 1)
			go s.handleConnection(conn)
		}
	}
}

// handleShutdown handles graceful shutdown signal
func (s *Server) handleShutdown(ctx context.Context) {
	<-ctx.Done()
	s.shutdownOnce.Do(func() {
		close(s.shutdownCh)
		// Close listener to unblock Accept() immediately
		if s.listener != nil {
			s.listener.Close()
		}
	})
}

// shutdown performs graceful shutdown
func (s *Server) shutdown() error {
	// Listener already closed in handleShutdown
	s.logger.Info("Waiting for active connections to finish...")

	// Wait for active connections with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All connections closed gracefully")
	case <-time.After(s.config.ShutdownTimeout):
		s.logger.Warn("Shutdown timeout reached, forcing shutdown",
			"active_connections", atomic.LoadInt32(&s.activeConns))
	}

	return nil
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		atomic.AddInt32(&s.activeConns, -1)
		s.wg.Done()
	}()

	remoteAddr := conn.RemoteAddr().String()
	s.logger.Info("New connection", "remote_addr", remoteAddr)

	// Generate challenge
	challenge, err := s.powService.GenerateChallenge()
	if err != nil {
		s.logger.Error("Failed to generate challenge", "error", err, "remote_addr", remoteAddr)
		s.sendError(conn, "Internal server error")
		return
	}

	// Send challenge to client
	challengeMsg := protocol.ChallengeMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeChallenge},
		Challenge:   challenge,
		Difficulty:  s.powService.GetDifficulty(),
	}

	if err := protocol.WriteMessage(conn, challengeMsg, s.config.WriteTimeout); err != nil {
		s.logger.Error("Failed to send challenge", "error", err, "remote_addr", remoteAddr)
		s.powService.InvalidateChallenge(challenge)
		return
	}

	s.logger.Debug("Challenge sent", "remote_addr", remoteAddr, "challenge", challenge)

	// Read proof from client
	var proofMsg protocol.ProofMessage
	if err := protocol.ReadMessage(conn, &proofMsg, s.config.ReadTimeout); err != nil {
		s.logger.Error("Failed to read proof", "error", err, "remote_addr", remoteAddr)
		s.powService.InvalidateChallenge(challenge)
		s.sendError(conn, "Failed to read proof")
		return
	}

	// CRITICAL: Verify that client is solving the challenge issued in THIS connection
	// This prevents replay attacks where client uses an old challenge from a different connection
	if proofMsg.Challenge != challenge {
		s.logger.Warn("Challenge mismatch - possible replay attack",
			"remote_addr", remoteAddr,
			"expected", challenge,
			"received", proofMsg.Challenge)
		s.powService.InvalidateChallenge(challenge)
		s.sendError(conn, "Challenge mismatch")
		return
	}

	// Verify proof
	valid, err := s.powService.VerifyProof(proofMsg.Challenge, proofMsg.Nonce)
	if err != nil {
		s.logger.Error("Failed to verify proof", "error", err, "remote_addr", remoteAddr)
		s.sendError(conn, fmt.Sprintf("Proof verification error: %v", err))
		return
	}

	if !valid {
		s.logger.Warn("Invalid proof", "remote_addr", remoteAddr)
		s.sendError(conn, "Invalid proof")
		return
	}

	s.logger.Info("Proof verified successfully", "remote_addr", remoteAddr)

	// Get and send quote
	quote := s.quotesService.GetRandomQuote()
	quoteMsg := protocol.QuoteMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeQuote},
		Quote:       quote,
	}

	if err := protocol.WriteMessage(conn, quoteMsg, s.config.WriteTimeout); err != nil {
		s.logger.Error("Failed to send quote", "error", err, "remote_addr", remoteAddr)
		return
	}

	s.logger.Info("Quote sent successfully", "remote_addr", remoteAddr)
}

// sendError sends an error message to the client
func (s *Server) sendError(conn net.Conn, message string) {
	errMsg := protocol.ErrorMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeError},
		Message:     message,
	}

	if err := protocol.WriteMessage(conn, errMsg, s.config.WriteTimeout); err != nil {
		s.logger.Error("Failed to send error message", "error", err)
	}
}

// GetActiveConnections returns the number of active connections
func (s *Server) GetActiveConnections() int32 {
	return atomic.LoadInt32(&s.activeConns)
}
