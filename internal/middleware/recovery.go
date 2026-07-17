package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/confuzeus/replyforge/internal/model"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &recoveryWriter{ResponseWriter: w}
			defer func() {
				if rec := recover(); rec != nil {
					PanicsTotal.Add(1)
					if logger != nil {
						requestID := RequestIDFromContext(r.Context())
						logger.Error("panic recovered",
							"request_id", requestID,
							"panic", rec,
							"stack", string(debug.Stack()),
						)
					}
					if !rw.wroteHeader {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(model.ErrorResponse{
							Error: model.ErrorDetail{
								Code:    "INTERNAL_ERROR",
								Message: "An unexpected error occurred",
							},
						})
					}
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

type recoveryWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rw *recoveryWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *recoveryWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
