package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"pow/internal/config"
	"pow/internal/pow"
	"pow/internal/quotes"
	"pow/internal/server"
)

func main() {
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting Word of Wisdom TCP server...")

	// Load configuration
	cfg := config.LoadServerConfig()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration", "error", err)
		log.Fatalf("Configuration validation failed: %v", err)
	}

	logger.Info("Configuration loaded",
		"host", cfg.Host,
		"port", cfg.Port,
		"difficulty", cfg.Difficulty,
		"max_connections", cfg.MaxConnections,
		"max_active_challenges", cfg.MaxActiveChallenges)

	// Initialize services
	powService := pow.NewSHA256HashcashServiceWithLimit(cfg.Difficulty, cfg.ChallengeTTL, cfg.MaxActiveChallenges)
	quotesService := quotes.NewInMemoryService()

	// Create server
	serverConfig := server.Config{
		Host:            cfg.Host,
		Port:            cfg.Port,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxConnections:  cfg.MaxConnections,
		ShutdownTimeout: cfg.ShutdownTimeout,
	}

	srv := server.NewServer(serverConfig, powService, quotesService, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		// Always send to channel, even if no error (nil means clean shutdown)
		errChan <- srv.ListenAndServe(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()

		// Wait for server to actually finish graceful shutdown
		logger.Info("Waiting for server to shut down gracefully...")
		if err := <-errChan; err != nil {
			logger.Error("Server shutdown error", "error", err)
			log.Fatal(err)
		}

	case err := <-errChan:
		// Server exited on its own (not due to signal)
		cancel()
		if err != nil {
			logger.Error("Server error", "error", err)
			log.Fatal(err)
		}
		// If err == nil, server shut down cleanly (shouldn't normally happen)
		logger.Info("Server exited without error")
	}

	logger.Info("Server stopped")
}
