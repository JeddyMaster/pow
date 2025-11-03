package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"pow/internal/client"
	"pow/internal/pow"
	"pow/internal/quotes"
	"pow/internal/server"
)

func TestIntegration_ClientServerFlow(t *testing.T) {
	// Disable verbose logging for tests
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Setup server
	powService := pow.NewSHA256HashcashService(1, 5*time.Minute) // Difficulty 1 for fast tests
	quotesService := quotes.NewInMemoryService()

	serverConfig := server.Config{
		Host:            "127.0.0.1",
		Port:            "0", // Random port
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 5 * time.Second,
	}

	// Use fixed port for testing
	serverConfig.Port = "18080"
	srv := server.NewServer(serverConfig, powService, quotesService, logger)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		srv.ListenAndServe(ctx)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Setup client
	clientPowService := pow.NewSHA256HashcashService(0, 0) // Client doesn't need TTL
	clientConfig := client.Config{
		ServerHost:     "127.0.0.1",
		ServerPort:     "18080",
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		SolveTimeout:   30 * time.Second,
	}

	c := client.NewClient(clientConfig, clientPowService, logger)

	// Test: Get a quote
	t.Run("SuccessfulQuoteRetrieval", func(t *testing.T) {
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

	// Test: Multiple requests
	t.Run("MultipleRequests", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			quote, err := c.RequestQuote(ctx)
			cancel()

			if err != nil {
				t.Fatalf("Request %d failed: %v", i+1, err)
			}

			if quote == "" {
				t.Errorf("Request %d returned empty quote", i+1)
			}
		}
	})

	// Cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)
}
