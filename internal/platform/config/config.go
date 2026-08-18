package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	ReservationTTL time.Duration
}

func Load() (Config, error) {
	addr := getenv("HTTP_ADDR", ":8080")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	ttlSeconds := getenvInt("RESERVATION_TTL_SECONDS", 30)
	if ttlSeconds <= 0 {
		return Config{}, errors.New("RESERVATION_TTL_SECONDS must be positive")
	}

	return Config{HTTPAddr: addr, DatabaseURL: databaseURL, ReservationTTL: time.Duration(ttlSeconds) * time.Second}, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
