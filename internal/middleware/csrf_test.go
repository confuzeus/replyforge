package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRF_AllowsSafeMethodsWithoutToken(t *testing.T) {
	csrf := NewCSRF(false)
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

func TestCSRF_BlocksMutationWithoutToken(t *testing.T) {
	csrf := NewCSRF(false)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "CSRF_INVALID", errObj["code"])
}

func TestCSRF_AllowsMutationWithMatchingCookieAndHeader(t *testing.T) {
	csrf := NewCSRF(false)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := csrf.Middleware(next)

	token := "test-csrf-token-value"
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/1", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set(csrfHeaderName, token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestCSRF_RejectsMismatchedToken(t *testing.T) {
	csrf := NewCSRF(false)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "cookie-token"})
	req.Header.Set(csrfHeaderName, "header-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRF_LoginIsExempt(t *testing.T) {
	csrf := NewCSRF(false)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRF_PublicPathsUnaffected(t *testing.T) {
	csrf := NewCSRF(false)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCSRF_IssueTokenSetsCookie(t *testing.T) {
	csrf := NewCSRF(true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/csrf", nil)
	rec := httptest.NewRecorder()
	csrf.IssueTokenHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.NotEmpty(t, body["csrf_token"])

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == csrfCookieName {
			found = c
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, body["csrf_token"], found.Value)
	assert.False(t, found.HttpOnly)
	assert.True(t, found.Secure)
	assert.Equal(t, "/api/v1/admin", found.Path)
}
