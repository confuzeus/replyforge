package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()
	assert.NotNil(t, sm)
	assert.Equal(t, 1*time.Hour, sm.ttl)
}

func TestCreateSession(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, err := sm.CreateSession()
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, sm.ValidateSession(token))
}

func TestCreateSession_UniqueTokens(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	t1, _ := sm.CreateSession()
	t2, _ := sm.CreateSession()
	assert.NotEqual(t, t1, t2)
	assert.NotEmpty(t, t1)
	assert.NotEmpty(t, t2)
	assert.Greater(t, len(t1), 30)
}

func TestValidateSession_Valid(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	assert.True(t, sm.ValidateSession(token))
}

func TestValidateSession_Expired(t *testing.T) {
	sm := NewSessionManager(1*time.Millisecond, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	time.Sleep(5 * time.Millisecond)
	assert.False(t, sm.ValidateSession(token))
}

func TestValidateSession_UnknownToken(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	assert.False(t, sm.ValidateSession("nonexistent-token"))
}

func TestDeleteSession(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	assert.True(t, sm.ValidateSession(token))

	sm.DeleteSession(token)
	assert.False(t, sm.ValidateSession(token))
}

func TestSetCookie(t *testing.T) {
	sm := NewSessionManager(24*time.Hour, true)
	defer sm.Stop()

	rec := httptest.NewRecorder()
	sm.SetCookie(rec, "test-token")

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]

	assert.Equal(t, "admin_session", c.Name)
	assert.Equal(t, "test-token", c.Value)
	assert.Equal(t, "/api/v1/admin", c.Path)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	assert.False(t, c.Expires.IsZero())
}

func TestClearCookie(t *testing.T) {
	sm := NewSessionManager(24*time.Hour, true)
	defer sm.Stop()

	rec := httptest.NewRecorder()
	sm.ClearCookie(rec)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]

	assert.Equal(t, "admin_session", c.Name)
	assert.Equal(t, "", c.Value)
	assert.Equal(t, -1, c.MaxAge)
	assert.True(t, c.Expires.Before(time.Now()))
}

func TestGetToken_Present(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: "my-token"})

	assert.Equal(t, "my-token", sm.GetToken(req))
}

func TestGetToken_Missing(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, "", sm.GetToken(req))
}

func TestAuthMiddleware_ValidSession(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()

	handler := sm.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestAuthMiddleware_NoCookie(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	handler := sm.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Not authenticated")
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	handler := sm.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: "invalid-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Not authenticated")
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	sm := NewSessionManager(1*time.Millisecond, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	time.Sleep(5 * time.Millisecond)

	handler := sm.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_SkipsForwardsNext(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()

	var gotRequestID string
	handler := sm.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get("X-Request-Id")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	req.Header.Set("X-Request-Id", "abc-123")
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "abc-123", gotRequestID)
}

func TestExpiredSession_ValidateReturnsFalse(t *testing.T) {
	sm := NewSessionManager(1*time.Millisecond, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	time.Sleep(5 * time.Millisecond)
	assert.False(t, sm.ValidateSession(token))
}

func TestValidSession_ValidateReturnsTrue(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, true)
	defer sm.Stop()

	token, _ := sm.CreateSession()
	assert.True(t, sm.ValidateSession(token))
}
