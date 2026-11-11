// Package config loads runtime settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds settings shared by every mode of the fuxi binary.
type Config struct {
	// DatabaseURL is a pgx connection string, e.g. postgres://user:pass@host:5432/db.
	DatabaseURL string
	// HTTPAddr is the listen address for API mode.
	HTTPAddr string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:     os.Getenv("FUXI_DATABASE_URL"),
		HTTPAddr:        getenv("FUXI_HTTP_ADDR", ":8080"),
		LogLevel:        getenv("FUXI_LOG_LEVEL", "info"),
		ShutdownTimeout: 15 * time.Second,
	}
	if v := os.Getenv("FUXI_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("FUXI_SHUTDOWN_TIMEOUT: %w", err)
		}
		c.ShutdownTimeout = d
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("FUXI_DATABASE_URL is required")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
