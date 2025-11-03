package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// Default server configuration values
	DefaultServerHost          = "0.0.0.0"
	DefaultServerPort          = "8080"
	DefaultDifficulty          = 2
	DefaultChallengeTTL        = 5 * time.Minute
	DefaultMaxActiveChallenges = 100000
	DefaultReadTimeout         = 30 * time.Second
	DefaultWriteTimeout        = 10 * time.Second
	DefaultMaxConnections      = 100
	DefaultShutdownTimeout     = 30 * time.Second

	// Default client configuration values
	DefaultClientHost         = "localhost"
	DefaultClientPort         = "8080"
	DefaultConnectTimeout     = 10 * time.Second
	DefaultClientReadTimeout  = 30 * time.Second
	DefaultClientWriteTimeout = 10 * time.Second
	DefaultSolveTimeout       = 5 * time.Minute

	// Configuration validation limits
	MinDifficulty          = 1
	MaxDifficulty          = 5
	MinMaxActiveChallenges = 100
	MinMaxConnections      = 1
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Host                string
	Port                string
	Difficulty          int
	ChallengeTTL        time.Duration
	MaxActiveChallenges int
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	MaxConnections      int
	ShutdownTimeout     time.Duration
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
		Host:                getEnv("SERVER_HOST", DefaultServerHost),
		Port:                getEnv("SERVER_PORT", DefaultServerPort),
		Difficulty:          getEnvInt("POW_DIFFICULTY", DefaultDifficulty),
		ChallengeTTL:        getEnvDuration("CHALLENGE_TTL", DefaultChallengeTTL),
		MaxActiveChallenges: getEnvInt("MAX_ACTIVE_CHALLENGES", DefaultMaxActiveChallenges),
		ReadTimeout:         getEnvDuration("READ_TIMEOUT", DefaultReadTimeout),
		WriteTimeout:        getEnvDuration("WRITE_TIMEOUT", DefaultWriteTimeout),
		MaxConnections:      getEnvInt("MAX_CONNECTIONS", DefaultMaxConnections),
		ShutdownTimeout:     getEnvDuration("SHUTDOWN_TIMEOUT", DefaultShutdownTimeout),
	}
}

// LoadClientConfig loads client configuration from environment variables
func LoadClientConfig() ClientConfig {
	return ClientConfig{
		ServerHost:     getEnv("SERVER_HOST", DefaultClientHost),
		ServerPort:     getEnv("SERVER_PORT", DefaultClientPort),
		ConnectTimeout: getEnvDuration("CONNECT_TIMEOUT", DefaultConnectTimeout),
		ReadTimeout:    getEnvDuration("READ_TIMEOUT", DefaultClientReadTimeout),
		WriteTimeout:   getEnvDuration("WRITE_TIMEOUT", DefaultClientWriteTimeout),
		SolveTimeout:   getEnvDuration("SOLVE_TIMEOUT", DefaultSolveTimeout),
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

// Validate validates server configuration
func (c ServerConfig) Validate() error {
	if c.ChallengeTTL <= 0 {
		return fmt.Errorf("CHALLENGE_TTL must be positive, got: %v", c.ChallengeTTL)
	}
	if c.Difficulty < MinDifficulty || c.Difficulty > MaxDifficulty {
		return fmt.Errorf("POW_DIFFICULTY must be between %d and %d, got: %d", MinDifficulty, MaxDifficulty, c.Difficulty)
	}
	if c.MaxActiveChallenges < MinMaxActiveChallenges {
		return fmt.Errorf("MAX_ACTIVE_CHALLENGES must be at least %d, got: %d", MinMaxActiveChallenges, c.MaxActiveChallenges)
	}
	if c.MaxConnections < MinMaxConnections {
		return fmt.Errorf("MAX_CONNECTIONS must be positive, got: %d", c.MaxConnections)
	}
	if c.ReadTimeout <= 0 {
		return fmt.Errorf("READ_TIMEOUT must be positive, got: %v", c.ReadTimeout)
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("WRITE_TIMEOUT must be positive, got: %v", c.WriteTimeout)
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got: %v", c.ShutdownTimeout)
	}
	return nil
}
