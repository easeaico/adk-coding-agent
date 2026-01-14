// Package config provides configuration loading and validation.
package config

import (
	"log"
	"os"
)

// Config holds the application configuration loaded from environment variables.
// All fields are required except WorkDir and DBType, which have sensible defaults.
type Config struct {
	DBType      string // Database type: "postgres" or "sqlite" (optional, defaults to "postgres")
	DatabaseURL string // PostgreSQL connection string or SQLite file path (required)
	APIKey      string // Google GenAI API key (required)
	WorkDir     string // Working directory for file operations (optional, defaults to current directory)
}

// Load loads configuration from environment variables.
func Load() Config {
	cfg := Config{
		DBType:      os.Getenv("DB_TYPE"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		WorkDir:     os.Getenv("WORK_DIR"),
	}

	// Set defaults
	if cfg.DBType == "" {
		cfg.DBType = "postgres"
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir, _ = os.Getwd()
	}

	// Validate DB_TYPE
	if cfg.DBType != "postgres" && cfg.DBType != "sqlite" {
		log.Fatalf("DB_TYPE must be 'postgres' or 'sqlite', got: %s", cfg.DBType)
	}

	// Validate required config
	if cfg.APIKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}
	if cfg.DatabaseURL == "" {
		if cfg.DBType == "postgres" {
			log.Fatal("DATABASE_URL environment variable is required (e.g., postgres://user:pass@localhost:5432/dbname)")
		} else {
			log.Fatal("DATABASE_URL environment variable is required (e.g., ./data.db or /path/to/database.db)")
		}
	}

	return cfg
}
