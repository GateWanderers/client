package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL string
	PasetoKey   []byte
	Port        string
}

// Load reads configuration from a .env file (if present) and environment variables.
func Load() (*Config, error) {
	// Load .env if it exists; ignore error if missing.
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	pasetoKeyHex := strings.TrimSpace(os.Getenv("PASETO_KEY"))
	if pasetoKeyHex == "" {
		return nil, fmt.Errorf("PASETO_KEY is required")
	}
	pasetoKey, err := hex.DecodeString(pasetoKeyHex)
	if err != nil {
		return nil, fmt.Errorf("PASETO_KEY must be a valid hex string: %w", err)
	}
	if len(pasetoKey) != 32 {
		return nil, fmt.Errorf("PASETO_KEY must decode to exactly 32 bytes, got %d", len(pasetoKey))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL: databaseURL,
		PasetoKey:   pasetoKey,
		Port:        port,
	}, nil
}
