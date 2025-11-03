package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"pow/internal/client"
	"pow/internal/config"
	"pow/internal/pow"
)

func main() {
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting Word of Wisdom TCP client...")

	// Load configuration
	cfg := config.LoadClientConfig()
	logger.Info("Configuration loaded",
		"server_host", cfg.ServerHost,
		"server_port", cfg.ServerPort)

	// Initialize PoW service (difficulty will be received from server)
	powService := pow.NewSHA256HashcashService(0, 0) // Difficulty not needed for client

	// Create client
	clientConfig := client.Config{
		ServerHost:     cfg.ServerHost,
		ServerPort:     cfg.ServerPort,
		ConnectTimeout: cfg.ConnectTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		SolveTimeout:   cfg.SolveTimeout,
	}

	c := client.NewClient(clientConfig, powService, logger)

	// Request quote
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SolveTimeout+cfg.ConnectTimeout+cfg.ReadTimeout)
	defer cancel()

	logger.Info("Requesting quote from server...")

	quote, err := c.RequestQuote(ctx)
	if err != nil {
		logger.Error("Failed to get quote", "error", err)
		log.Fatal(err)
	}

	// Print quote to user
	separator := "================================================================================"
	fmt.Println("\n" + separator)
	fmt.Println("Quote of the Day:")
	fmt.Println(quote)
	fmt.Println(separator + "\n")

	logger.Info("Quote retrieved successfully")
}
