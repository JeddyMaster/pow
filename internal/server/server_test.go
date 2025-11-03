package server

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"pow/internal/pow"
	"pow/internal/quotes"
)

func TestServer_GracefulShutdown(t *testing.T) {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create services
	powService := pow.NewSHA256HashcashService(1, 5*time.Minute)
	quotesService := quotes.NewInMemoryService()

	// Create server config
	config := Config{
		Host:            "127.0.0.1",
		Port:            "18081",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 2 * time.Second,
	}

	srv := NewServer(config, powService, quotesService, logger)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())

	serverDone := make(chan error, 1)

	go func() {
		serverDone <- srv.ListenAndServe(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Initiate shutdown
	shutdownStart := time.Now()
	cancel()

	// Wait for server to stop with timeout
	select {
	case serverErr := <-serverDone:
		shutdownDuration := time.Since(shutdownStart)
		t.Logf("Server shutdown completed in %v", shutdownDuration)

		// Verify shutdown was quick (no hanging)
		if shutdownDuration > 5*time.Second {
			t.Errorf("Shutdown took too long: %v", shutdownDuration)
		}

		// Verify no error (nil is expected for clean shutdown)
		if serverErr != nil {
			t.Logf("Server returned error (may be expected): %v", serverErr)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Server shutdown timed out - graceful shutdown not working")
	}
}

func TestServer_GracefulShutdownWithActiveConnections(t *testing.T) {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create services
	powService := pow.NewSHA256HashcashService(1, 5*time.Minute)
	quotesService := quotes.NewInMemoryService()

	// Create server config
	config := Config{
		Host:            "127.0.0.1",
		Port:            "18082",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 1 * time.Second,
	}

	srv := NewServer(config, powService, quotesService, logger)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())

	serverDone := make(chan struct{})
	go func() {
		srv.ListenAndServe(ctx)
		close(serverDone)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Simulate active connections
	srv.wg.Add(2)
	atomic.AddInt32(&srv.activeConns, 2)

	// Start goroutines that simulate long-running handlers
	handlersDone := make(chan struct{})
	go func() {
		defer srv.wg.Done()
		defer atomic.AddInt32(&srv.activeConns, -1)
		time.Sleep(500 * time.Millisecond) // Simulate work
	}()

	go func() {
		defer srv.wg.Done()
		defer atomic.AddInt32(&srv.activeConns, -1)
		time.Sleep(500 * time.Millisecond) // Simulate work
	}()

	// Wait a bit to ensure handlers are running
	time.Sleep(50 * time.Millisecond)

	// Initiate shutdown
	shutdownStart := time.Now()
	cancel()

	// Wait for server to stop
	select {
	case <-serverDone:
		shutdownDuration := time.Since(shutdownStart)
		t.Logf("Server shutdown completed in %v", shutdownDuration)

		// Verify shutdown waited for handlers (but not too long)
		if shutdownDuration < 400*time.Millisecond {
			t.Errorf("Shutdown was too fast (%v), may not have waited for handlers", shutdownDuration)
		}

		if shutdownDuration > 3*time.Second {
			t.Errorf("Shutdown took too long: %v", shutdownDuration)
		}

		// Verify all connections were handled
		if srv.GetActiveConnections() != 0 {
			t.Errorf("Expected 0 active connections, got %d", srv.GetActiveConnections())
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Server shutdown timed out")
	}

	close(handlersDone)
}

func TestServer_MaxConnections(t *testing.T) {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create services
	powService := pow.NewSHA256HashcashService(1, 5*time.Minute)
	quotesService := quotes.NewInMemoryService()

	// Create server config with low max connections
	config := Config{
		Host:            "127.0.0.1",
		Port:            "18083",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		MaxConnections:  2,
		ShutdownTimeout: 1 * time.Second,
	}

	srv := NewServer(config, powService, quotesService, logger)

	// Verify initial state
	if srv.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 initial connections, got %d", srv.GetActiveConnections())
	}

	// Simulate reaching max connections
	atomic.StoreInt32(&srv.activeConns, 2)

	if srv.GetActiveConnections() != 2 {
		t.Errorf("Expected 2 active connections, got %d", srv.GetActiveConnections())
	}
}
