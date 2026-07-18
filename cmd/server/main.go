package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"expvar"
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
	"github.com/confuzeus/replyforge/internal/templates"
	"github.com/confuzeus/replyforge/migrations"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg := config.Load()
	startTime := time.Now()

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

	adminHandler := handler.NewAdminHandler(handler.AdminHandlerDependencies{
		Service:      commentService,
		AdminPage:    templates.AdminPage,
		PasswordHash: cfg.AdminPasswordHash,
		Logger:       logger,
	})

	mux := http.NewServeMux()
	commentHandler.RegisterRoutes(mux)
	adminHandler.RegisterRoutes(mux)

	corsCfg := middleware.NewCORSConfig(cfg.AllowedOrigins)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPM, cfg.RateLimitBurst, logger)

	mux.HandleFunc("GET /health", healthHandler(db, cfg.Version, startTime))
	mux.Handle("GET /debug/vars", expvar.Handler())

	deps := middlewareDeps{
		Logger:      logger,
		CORSConfig:  corsCfg,
		RateLimiter:  rateLimiter,
	}

	wrappedHandler := setupMiddleware(mux, deps)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      wrappedHandler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logger.Info("starting replyforge server",
			"port", cfg.Port,
			"version", cfg.Version,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	uptime := time.Since(startTime).Seconds()
	logger.Info("shutting down server",
		"signal", sig.String(),
		"uptime_seconds", uptime,
		"comments_created_total", middleware.CommentsCreatedTotal.Value(),
		"turnstile_verifications_total", middleware.TurnstileVerificationsTotal.Value(),
		"turnstile_failed_total", middleware.TurnstileFailedTotal.Value(),
		"rate_limit_hits_total", middleware.RateLimitHitsTotal.Value(),
		"validation_errors_total", middleware.ValidationErrorsTotal.Value(),
		"panics_total", middleware.PanicsTotal.Value(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	shutdownDone := make(chan struct{})
	go func() {
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("forced shutdown", "error", err)
		}
		rateLimiter.Stop()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
	case sig := <-quit:
		logger.Warn("received second signal, forcing shutdown", "signal", sig.String())
		cancel()
		<-shutdownDone
	}

	logger.Info("server stopped")
}

func healthHandler(db *sql.DB, version string, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Seconds()
		result := map[string]interface{}{
			"status":         "healthy",
			"version":        version,
			"uptime_seconds": uptime,
		}

		dbStart := time.Now()
		dbErr := db.Ping()
		dbLatency := time.Since(dbStart).Milliseconds()

		dbCheck := map[string]interface{}{
			"status":      "connected",
			"response_ms": dbLatency,
		}
		if dbErr != nil {
			dbCheck["status"] = "disconnected"
			dbCheck["error"] = dbErr.Error()
			result["status"] = "degraded"
		}

		result["checks"] = map[string]interface{}{
			"database": dbCheck,
		}

		w.Header().Set("Content-Type", "application/json")
		statusCode := http.StatusOK
		if dbErr != nil {
			statusCode = http.StatusServiceUnavailable
		}
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(result)
	}
}

type middlewareDeps struct {
	Logger      *slog.Logger
	CORSConfig  *middleware.CORSConfig
	RateLimiter *middleware.RateLimiter
}

func setupMiddleware(handler http.Handler, deps middlewareDeps) http.Handler {
	handler = middleware.Recovery(deps.Logger)(handler)
	handler = middleware.Logging(deps.Logger)(handler)
	handler = middleware.CORS(deps.CORSConfig)(handler)
	handler = middleware.RateLimit(deps.RateLimiter)(handler)
	return handler
}
