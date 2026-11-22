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
	// Auth configures sign-in for API mode (ADR-0018).
	Auth Auth
	// WebDir is the built PWA (web/dist) that API mode serves at /. Empty
	// means the API serves no client; in development Vite serves it instead.
	WebDir string
}

// Auth modes.
const (
	AuthCloudflare = "cloudflare" // verify Cloudflare Access tokens (production)
	AuthDev        = "dev"        // trust a fixed email or the X-Fuxi-Dev-Email header (local only)
)

// Auth configures how API requests are authenticated.
type Auth struct {
	// Mode is AuthCloudflare (the default) or AuthDev.
	Mode string
	// AccessTeamDomain is the Cloudflare Access team URL, e.g. https://fuxi.cloudflareaccess.com.
	AccessTeamDomain string
	// AccessAudience is the Access application's AUD tag.
	AccessAudience string
	// DevEmail is the user every request signs in as in dev mode.
	DevEmail string
	// OwnerEmail gets the owner role when its account is created.
	OwnerEmail string
}

// Validate reports settings API mode can't run with.
func (a Auth) Validate() error {
	switch a.Mode {
	case AuthCloudflare:
		if a.AccessTeamDomain == "" || a.AccessAudience == "" {
			return errors.New("FUXI_AUTH=cloudflare needs FUXI_ACCESS_TEAM_DOMAIN and FUXI_ACCESS_AUD")
		}
	case AuthDev:
	default:
		return fmt.Errorf("FUXI_AUTH must be %s or %s, not %q", AuthCloudflare, AuthDev, a.Mode)
	}
	return nil
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:     os.Getenv("FUXI_DATABASE_URL"),
		HTTPAddr:        getenv("FUXI_HTTP_ADDR", ":8080"),
		LogLevel:        getenv("FUXI_LOG_LEVEL", "info"),
		ShutdownTimeout: 15 * time.Second,
		WebDir:          os.Getenv("FUXI_WEB_DIR"),
		Auth: Auth{
			Mode:             getenv("FUXI_AUTH", AuthCloudflare),
			AccessTeamDomain: os.Getenv("FUXI_ACCESS_TEAM_DOMAIN"),
			AccessAudience:   os.Getenv("FUXI_ACCESS_AUD"),
			DevEmail:         getenv("FUXI_DEV_EMAIL", "dev@fuxi.localhost"),
			OwnerEmail:       os.Getenv("FUXI_OWNER_EMAIL"),
		},
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
