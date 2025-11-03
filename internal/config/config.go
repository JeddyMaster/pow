package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Host            string
	Port            string
	Difficulty      int
	ChallengeTTL    time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxConnections  int
	ShutdownTimeout time.Duration
}

// ClientConfig holds client configuration
type ClientConfig struct {
	ServerHost     string
	ServerPort     string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	SolveTimeout   time.Duration
}

// LoadServerConfig loads server configuration from environment variables
func LoadServerConfig() ServerConfig {
	return ServerConfig{
		Host:            getEnv("SERVER_HOST", "0.0.0.0"),
		Port:            getEnv("SERVER_PORT", "8080"),
		Difficulty:      getEnvInt("POW_DIFFICULTY", 2),
		ChallengeTTL:    getEnvDuration("CHALLENGE_TTL", 5*time.Minute),
		ReadTimeout:     getEnvDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    getEnvDuration("WRITE_TIMEOUT", 10*time.Second),
		MaxConnections:  getEnvInt("MAX_CONNECTIONS", 100),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
	}
}

// LoadClientConfig loads client configuration from environment variables
func LoadClientConfig() ClientConfig {
	return ClientConfig{
		ServerHost:     getEnv("SERVER_HOST", "localhost"),
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		ConnectTimeout: getEnvDuration("CONNECT_TIMEOUT", 10*time.Second),
		ReadTimeout:    getEnvDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:   getEnvDuration("WRITE_TIMEOUT", 10*time.Second),
		SolveTimeout:   getEnvDuration("SOLVE_TIMEOUT", 5*time.Minute),
	}
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int or returns default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		fmt.Printf("Warning: invalid value for %s, using default: %d\n", key, defaultValue)
	}
	return defaultValue
}

// getEnvDuration gets environment variable as duration or returns default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		fmt.Printf("Warning: invalid duration for %s, using default: %s\n", key, defaultValue)
	}
	return defaultValue
}
