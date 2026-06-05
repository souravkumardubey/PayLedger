package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         env("PORT", "8080"),
			ReadTimeout:  envInt("READ_TIMEOUT", 10),
			WriteTimeout: envInt("WRITE_TIMEOUT", 10),
		},
		Database: DatabaseConfig{
			Driver: "postgres",
			DSN:    env("DB_DSN", "postgres://postgres:postgres@localhost:5432/transaction_engine?sslmode=disable"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
