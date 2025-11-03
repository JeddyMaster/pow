package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"pow/internal/client"
	"pow/internal/pow"
	"pow/internal/quotes"
	"pow/internal/server"
	"pow/pkg/protocol"
)

// TestE2E_FullFlow tests the complete challenge -> proof -> quote flow
func TestE2E_FullFlow(t *testing.T) {
	// Setup logger (suppress output in tests)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Setup server
	difficulty := 1 // Low difficulty for fast tests
	powService := pow.NewSHA256HashcashService(difficulty, 5*time.Minute)
	quotesService := quotes.NewInMemoryService()

	serverConfig := server.Config{
		Host:            "127.0.0.1",
		Port:            "18090",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.NewServer(serverConfig, powService, quotesService, logger)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})
	go func() {
		close(serverReady)
		srv.ListenAndServe(ctx)
	}()

	<-serverReady
	time.Sleep(100 * time.Millisecond) // Give server time to bind

	// Setup client
	clientPowService := pow.NewSHA256HashcashService(0, 0)
	clientConfig := client.Config{
		ServerHost:     "127.0.0.1",
		ServerPort:     "18090",
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		SolveTimeout:   30 * time.Second,
	}

	c := client.NewClient(clientConfig, clientPowService, logger)

	// Test: Successful flow
	t.Run("SuccessfulFlow", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		quote, err := c.RequestQuote(ctx)
		if err != nil {
			t.Fatalf("Failed to get quote: %v", err)
		}

		if quote == "" {
			t.Error("Quote should not be empty")
		}

		t.Logf("Received quote: %s", quote)
	})

	// Test: Invalid proof should be rejected
	t.Run("InvalidProofRejected", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:18090", 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		// Read challenge
		var challengeMsg protocol.ChallengeMessage
		if err := protocol.ReadMessage(conn, &challengeMsg, 10*time.Second); err != nil {
			t.Fatalf("Failed to read challenge: %v", err)
		}

		// Send invalid proof
		proofMsg := protocol.ProofMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeProof},
			Challenge:   challengeMsg.Challenge,
			Nonce:       "invalid_nonce_12345",
		}

		if err := protocol.WriteMessage(conn, proofMsg, 10*time.Second); err != nil {
			t.Fatalf("Failed to send proof: %v", err)
		}

		// Should receive error message
		var response map[string]interface{}
		if err := protocol.ReadMessage(conn, &response, 10*time.Second); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		msgType, ok := response["type"].(string)
		if !ok || msgType != "error" {
			t.Errorf("Expected error message, got type: %v", msgType)
		}

		t.Logf("Server correctly rejected invalid proof")
	})

	// Test: Wrong challenge should be rejected (replay attack prevention)
	t.Run("WrongChallengeRejected", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:18090", 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		// Read challenge
		var challengeMsg protocol.ChallengeMessage
		if err := protocol.ReadMessage(conn, &challengeMsg, 10*time.Second); err != nil {
			t.Fatalf("Failed to read challenge: %v", err)
		}

		// Send proof for a different challenge
		wrongChallenge := "1234567890:abcdef"
		solveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		nonce, err := clientPowService.SolveChallenge(solveCtx, wrongChallenge, difficulty)
		if err != nil {
			t.Fatalf("Failed to solve challenge: %v", err)
		}

		proofMsg := protocol.ProofMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MsgTypeProof},
			Challenge:   wrongChallenge, // Wrong challenge!
			Nonce:       nonce,
		}

		if err := protocol.WriteMessage(conn, proofMsg, 10*time.Second); err != nil {
			t.Fatalf("Failed to send proof: %v", err)
		}

		// Should receive error message
		var response map[string]interface{}
		if err := protocol.ReadMessage(conn, &response, 10*time.Second); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		msgType, ok := response["type"].(string)
		if !ok || msgType != "error" {
			t.Errorf("Expected error message, got type: %v", msgType)
		}

		errMsg, ok := response["message"].(string)
		if !ok {
			t.Error("Error message should have 'message' field")
		}

		if errMsg != "Challenge mismatch" {
			t.Errorf("Expected 'Challenge mismatch' error, got: %s", errMsg)
		}

		t.Logf("Server correctly rejected wrong challenge")
	})

	// Test: Multiple concurrent requests
	t.Run("ConcurrentRequests", func(t *testing.T) {
		const numRequests = 5
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				quote, err := c.RequestQuote(ctx)
				if err != nil {
					results <- err
					return
				}

				if quote == "" {
					results <- fmt.Errorf("request %d: empty quote", id)
					return
				}

				results <- nil
			}(i)
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			if err := <-results; err != nil {
				t.Errorf("Concurrent request failed: %v", err)
			}
		}

		t.Logf("All %d concurrent requests succeeded", numRequests)
	})

	// Cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestE2E_Timeout tests that client properly handles timeouts
func TestE2E_Timeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Setup server with high difficulty to ensure timeout
	difficulty := 5 // Very high difficulty - nearly impossible to solve quickly
	powService := pow.NewSHA256HashcashService(difficulty, 5*time.Minute)
	quotesService := quotes.NewInMemoryService()

	serverConfig := server.Config{
		Host:            "127.0.0.1",
		Port:            "18091", // Different port from main test
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.NewServer(serverConfig, powService, quotesService, logger)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})
	go func() {
		close(serverReady)
		srv.ListenAndServe(ctx)
	}()

	<-serverReady
	time.Sleep(100 * time.Millisecond) // Give server time to bind

	// Setup client with very short timeout
	clientPowService := pow.NewSHA256HashcashService(0, 0)
	clientConfig := client.Config{
		ServerHost:     "127.0.0.1",
		ServerPort:     "18091",
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		SolveTimeout:   100 * time.Millisecond, // Too short to solve difficulty 5
	}

	c := client.NewClient(clientConfig, clientPowService, logger)

	// This should timeout while solving the challenge
	requestCtx, requestCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer requestCancel()

	_, err := c.RequestQuote(requestCtx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err != nil {
		t.Logf("Client correctly returned timeout error: %v", err)
	}

	// Cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)
}
