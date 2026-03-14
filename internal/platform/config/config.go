package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config keeps process-level settings loaded from environment variables.
type Config struct {
	BotToken             string
	WebhookSecret        string
	HTTPAddr             string
	DatabaseURL          string
	RedisURL             string
	StreamingAPIURL      string
	StreamingAPIKey      string
	AccessLinkTTLMinutes int
	ReviewDelayHours     int
	RateLimitPerMinute   int
	LogLevel             string
	Environment          string
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:             os.Getenv("BOT_TOKEN"),
		WebhookSecret:        os.Getenv("WEBHOOK_SECRET"),
		HTTPAddr:             getenvDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:          getenvDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/streamingbot?sslmode=disable"),
		RedisURL:             getenvDefault("REDIS_URL", "redis://localhost:6379/0"),
		StreamingAPIURL:      os.Getenv("STREAMING_API_URL"),
		StreamingAPIKey:      os.Getenv("STREAMING_API_KEY"),
		AccessLinkTTLMinutes: getenvInt("ACCESS_LINK_TTL_MINUTES", 1440),
		ReviewDelayHours:     getenvInt("REVIEW_DELAY_HOURS", 24),
		RateLimitPerMinute:   getenvInt("RATE_LIMIT_PER_MINUTE", 10),
		LogLevel:             getenvDefault("LOG_LEVEL", "info"),
		Environment:          getenvDefault("ENVIRONMENT", "local"),
	}

	if cfg.BotToken == "" {
		if cfg.Environment == "prod" || cfg.Environment == "production" {
			return Config{}, fmt.Errorf("BOT_TOKEN is required in production")
		}
		cfg.BotToken = "dev-bot-token"
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
