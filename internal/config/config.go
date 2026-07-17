package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabasePath       string
	LogLevel           slog.Level
	TurnstileSecretKey string
	HashIDSalt         string
	AllowedOrigins     []string
	RateLimitRPM       int
	RateLimitBurst     int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:               envOrDefault("PORT", "8080"),
		DatabasePath:       envOrDefault("DATABASE_PATH", "data/comments.db"),
		LogLevel:           parseLogLevel(envOrDefault("LOG_LEVEL", "info")),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		HashIDSalt:         envOrDefault("HASHID_SALT", "default-salt"),
		AllowedOrigins:     parseAllowedOrigins(envOrDefault("ALLOWED_ORIGINS", "*")),
		RateLimitRPM:       envOrDefaultInt("RATE_LIMIT_RPM", 10),
		RateLimitBurst:     envOrDefaultInt("RATE_LIMIT_BURST", 15),
		ReadTimeout:        envOrDefaultDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:       envOrDefaultDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:        envOrDefaultDuration("IDLE_TIMEOUT", 60*time.Second),
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.TurnstileSecretKey == "" {
		return fmt.Errorf("TURNSTILE_SECRET_KEY is required")
	}
	return nil
}

func parseLogLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return level
}

func parseAllowedOrigins(s string) []string {
	if s == "*" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
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

func envOrDefaultDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
