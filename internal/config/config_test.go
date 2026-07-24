package config

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear all relevant env vars before test
	for _, k := range os.Environ() {
		key := strings.SplitN(k, "=", 2)[0]
		os.Unsetenv(key)
	}

	cfg := Load()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Port", cfg.Port, "8080"},
		{"DatabasePath", cfg.DatabasePath, "data/comments.db"},
		{"LogLevel", cfg.LogLevel, slog.LevelInfo},
		{"AllowedOrigins", cfg.AllowedOrigins, []string{"*"}},
		{"RateLimitRPM", cfg.RateLimitRPM, 10},
		{"RateLimitBurst", cfg.RateLimitBurst, 15},
		{"ReadTimeout", cfg.ReadTimeout, 10 * time.Second},
		{"WriteTimeout", cfg.WriteTimeout, 10 * time.Second},
		{"IdleTimeout", cfg.IdleTimeout, 60 * time.Second},
		{"ShutdownTimeout", cfg.ShutdownTimeout, 30 * time.Second},
		{"Version", cfg.Version, "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := tt.got.([]string); ok {
				exp := tt.expected.([]string)
				if len(got) != len(exp) || got[0] != exp[0] {
					t.Errorf("got %v, want %v", got, exp)
				}
				return
			}
			if tt.got != tt.expected {
				t.Errorf("got %v, want %v", tt.got, tt.expected)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	for _, k := range os.Environ() {
		key := strings.SplitN(k, "=", 2)[0]
		os.Unsetenv(key)
	}

	os.Setenv("PORT", "3000")
	os.Setenv("DATABASE_PATH", "/tmp/test.db")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("TURNSTILE_SECRET_KEY", "test-secret")
	os.Setenv("ALLOWED_ORIGINS", "https://foo.com,https://bar.com")
	os.Setenv("RATE_LIMIT_RPM", "30")
	os.Setenv("RATE_LIMIT_BURST", "5")
	os.Setenv("READ_TIMEOUT", "5s")
	os.Setenv("WRITE_TIMEOUT", "15s")
	os.Setenv("IDLE_TIMEOUT", "120s")
	os.Setenv("SHUTDOWN_TIMEOUT", "10s")
	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
	if cfg.DatabasePath != "/tmp/test.db" {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "/tmp/test.db")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
	if cfg.TurnstileSecretKey != "test-secret" {
		t.Errorf("TurnstileSecretKey = %q, want %q", cfg.TurnstileSecretKey, "test-secret")
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://foo.com" || cfg.AllowedOrigins[1] != "https://bar.com" {
		t.Errorf("AllowedOrigins = %v, want [https://foo.com https://bar.com]", cfg.AllowedOrigins)
	}
	if cfg.RateLimitRPM != 30 {
		t.Errorf("RateLimitRPM = %d, want 30", cfg.RateLimitRPM)
	}
	if cfg.RateLimitBurst != 5 {
		t.Errorf("RateLimitBurst = %d, want 5", cfg.RateLimitBurst)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.Version != "dev" {
		t.Errorf("Version = %q, want %q", cfg.Version, "dev")
	}
}

func TestValidateRejectsEmptySecretKey(t *testing.T) {
	cfg := &Config{CaptchaProvider: "turnstile", TurnstileSecretKey: ""}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty TURNSTILE_SECRET_KEY, got nil")
	}
}

func TestValidatePassesWithSecretKey(t *testing.T) {
	cfg := &Config{CaptchaProvider: "turnstile", TurnstileSecretKey: "not-empty"}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_PCaptchaNoSecretKey(t *testing.T) {
	cfg := &Config{CaptchaProvider: "pcaptcha", TurnstileSecretKey: ""}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected pcaptcha to not require TURNSTILE_SECRET_KEY, got: %v", err)
	}
}

func TestValidate_InvalidProvider(t *testing.T) {
	cfg := &Config{CaptchaProvider: "bogus"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid CAPTCHA_PROVIDER, got nil")
	}
}

func TestLoad_PCaptchaDefaults(t *testing.T) {
	os.Setenv("CAPTCHA_PROVIDER", "pcaptcha")
	defer os.Unsetenv("CAPTCHA_PROVIDER")

	cfg := Load()
	if cfg.CaptchaProvider != "pcaptcha" {
		t.Errorf("CaptchaProvider = %q, want %q", cfg.CaptchaProvider, "pcaptcha")
	}
	if cfg.CaptchaWoodall != "md" {
		t.Errorf("CaptchaWoodall = %q, want %q", cfg.CaptchaWoodall, "md")
	}
	if cfg.CaptchaRounds != 2 {
		t.Errorf("CaptchaRounds = %d, want 2", cfg.CaptchaRounds)
	}
}
