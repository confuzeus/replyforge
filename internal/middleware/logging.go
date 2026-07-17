package middleware

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			clientIP := ExtractClientIP(r)
			start := time.Now()
			rw := &loggingWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			statusCode := rw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			hash := sha256.Sum256([]byte(clientIP))
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"client_ip", fmt.Sprintf("%x", hash),
				"user_agent", r.UserAgent(),
			)
		})
	}
}

type loggingWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingWriter) WriteHeader(code int) {
	if lw.statusCode == 0 {
		lw.statusCode = code
		lw.ResponseWriter.WriteHeader(code)
	}
}

func (lw *loggingWriter) Write(b []byte) (int, error) {
	if lw.statusCode == 0 {
		lw.statusCode = http.StatusOK
	}
	return lw.ResponseWriter.Write(b)
}
