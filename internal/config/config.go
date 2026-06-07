package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:        getenv("PORT", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisAddr:   os.Getenv("REDIS_ADDR"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR is required")
	}

	return cfg, nil
}

func getenv(k, fallback string) string {
	value := os.Getenv(k)

	if value == "" {
		return fallback
	}
	return value
}
