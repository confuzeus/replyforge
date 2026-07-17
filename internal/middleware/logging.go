package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func generateRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString(make([]byte, 16))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil || r.URL.Path == "/health" || r.URL.Path == "/debug/vars" {
				next.ServeHTTP(w, r)
				return
			}
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			r = r.WithContext(ctx)
			w.Header().Set("X-Request-ID", requestID)
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
				"request_id", requestID,
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
