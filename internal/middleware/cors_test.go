package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS_AllowedOrigin(t *testing.T) {
	cors := NewCORSConfig([]string{"https://example.com"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "86400", rec.Header().Get("Access-Control-Max-Age"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	cors := NewCORSConfig([]string{"https://example.com"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Preflight(t *testing.T) {
	cors := NewCORSConfig([]string{"https://example.com"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/comments", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	cors := NewCORSConfig([]string{"*"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NoOriginHeader(t *testing.T) {
	cors := NewCORSConfig([]string{"https://example.com"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	cors := NewCORSConfig([]string{"https://example.com", "https://blog.example.com"})
	handler := cors.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	req.Header.Set("Origin", "https://blog.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://blog.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}
