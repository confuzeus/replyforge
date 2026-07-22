package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRF_AllowsSafeMethods(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRF_AllowsSameOriginMutation(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRF_BlocksCrossOriginMutation(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "CSRF_INVALID", errObj["code"])
}

func TestCSRF_AllowsNonBrowserRequests(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRF_PublicPathsUnaffected(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCSRF_BlocksCrossOriginDelete(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/1", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRF_OriginFallbackBlocksCrossOrigin(t *testing.T) {
	csrf := NewCSRF()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Host = "admin.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
