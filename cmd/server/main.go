package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confuzeus/replyforge/internal/config"
	"github.com/confuzeus/replyforge/internal/handler"
	"github.com/confuzeus/replyforge/internal/middleware"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	"github.com/confuzeus/replyforge/internal/sanitizer"
	"github.com/confuzeus/replyforge/internal/service"
	"github.com/confuzeus/replyforge/migrations"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite3", cfg.DatabasePath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := config.RunMigrations(db, migrations.SQL); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	repo := repository.NewCommentRepository(db)

	displayIDGen := model.NewDisplayIDGenerator()
	turnstileVerifier := service.NewTurnstileVerifier(cfg.TurnstileSecretKey)
	inputSanitizer := sanitizer.NewSanitizer()

	commentService := service.NewCommentService(service.ServiceDependencies{
		Repository:   repo,
		DisplayIDGen: displayIDGen,
		Turnstile:    turnstileVerifier,
		Sanitizer:    inputSanitizer,
		Logger:       logger,
	})

	commentHandler := handler.NewCommentHandler(handler.HandlerDependencies{
		Service: commentService,
		Logger:  logger,
	})

	mux := http.NewServeMux()
	commentHandler.RegisterRoutes(mux)

	corsCfg := middleware.NewCORSConfig(cfg.AllowedOrigins)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPM, cfg.RateLimitBurst)

	mux.Handle("POST /api/v1/comments", rateLimiter.Middleware(http.HandlerFunc(commentHandler.Create)))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	wrappedHandler := corsCfg.Middleware(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      wrappedHandler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logger.Info("starting replyforge server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutting down server", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}

	rateLimiter.Stop()

	logger.Info("server stopped")
}
