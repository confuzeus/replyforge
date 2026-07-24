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
	AllowedOrigins     []string
	RateLimitRPM       int
	RateLimitBurst     int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
	Version            string
	AdminPasswordHash  string
	AdminSessionTTL    time.Duration
	AdminSessionSecure bool

	CaptchaProvider string
	CaptchaWoodall  string
	CaptchaRounds   int

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPTo       string
}

var Version = "dev"

func Load() *Config {
	_ = godotenv.Load()

	version := Version

	cfg := &Config{
		Port:               envOrDefault("PORT", "8080"),
		DatabasePath:       envOrDefault("DATABASE_PATH", "data/comments.db"),
		LogLevel:           parseLogLevel(envOrDefault("LOG_LEVEL", "info")),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		AllowedOrigins:     parseAllowedOrigins(envOrDefault("ALLOWED_ORIGINS", "*")),
		RateLimitRPM:       envOrDefaultInt("RATE_LIMIT_RPM", 10),
		RateLimitBurst:     envOrDefaultInt("RATE_LIMIT_BURST", 15),
		ReadTimeout:        envOrDefaultDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:       envOrDefaultDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:        envOrDefaultDuration("IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:    envOrDefaultDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
		Version:            version,
		AdminPasswordHash:  os.Getenv("ADMIN_PASSWORD_HASH"),
		AdminSessionTTL:    envOrDefaultDuration("ADMIN_SESSION_TTL", 24*time.Hour),
		AdminSessionSecure: envOrDefaultBool("ADMIN_SESSION_SECURE", true),

		CaptchaProvider: envOrDefault("CAPTCHA_PROVIDER", "turnstile"),
		CaptchaWoodall:  envOrDefault("CAPTCHA_WOODALL", "md"),
		CaptchaRounds:   envOrDefaultInt("CAPTCHA_ROUNDS", 2),

		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     envOrDefaultInt("SMTP_PORT", 587),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),
		SMTPTo:       os.Getenv("SMTP_TO"),
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.CaptchaProvider != "turnstile" && c.CaptchaProvider != "pcaptcha" {
		return fmt.Errorf("CAPTCHA_PROVIDER must be 'turnstile' or 'pcaptcha', got: %s", c.CaptchaProvider)
	}
	if c.CaptchaProvider == "turnstile" && c.TurnstileSecretKey == "" {
		return fmt.Errorf("TURNSTILE_SECRET_KEY is required when CAPTCHA_PROVIDER=turnstile")
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

func envOrDefaultBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
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
