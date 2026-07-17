package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogging_StatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		handler    http.Handler
	}{
		{
			name:       "200 OK",
			statusCode: http.StatusOK,
			handler:    okHandler(),
		},
		{
			name:       "201 Created",
			statusCode: http.StatusCreated,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}),
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			}),
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
			handler := Logging(logger)(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.statusCode, rec.Code)

			var logEntry map[string]interface{}
			err := json.NewDecoder(&buf).Decode(&logEntry)
			require.NoError(t, err)
			assert.Equal(t, float64(tt.statusCode), logEntry["status"])
		})
	}
}

func TestLogging_ResponseBody(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	bodyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	})
	handler := Logging(logger)(bodyHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(body))
}

func TestLogging_HealthEndpoint(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := Logging(logger)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, buf.Bytes())
}

func TestLogging_NilLogger(t *testing.T) {
	handler := Logging(nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		handler.ServeHTTP(rec, req)
	})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLogging_Fields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := Logging(logger)(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var logEntry map[string]interface{}
	err := json.NewDecoder(&buf).Decode(&logEntry)
	require.NoError(t, err)

	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "/api/v1/comments", logEntry["path"])
	assert.Equal(t, float64(http.StatusOK), logEntry["status"])
	assert.Contains(t, logEntry, "duration_ms")
	assert.Contains(t, logEntry, "client_ip")
	assert.Equal(t, "TestAgent/1.0", logEntry["user_agent"])
}
