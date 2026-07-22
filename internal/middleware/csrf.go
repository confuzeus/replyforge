package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/confuzeus/replyforge/internal/model"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormField  = "csrf_token"
)

// CSRF protects state-changing admin requests that rely on the session cookie.
//
// Pattern: double-submit cookie.
//  1. GET /api/v1/admin/csrf issues a random token as a non-HttpOnly cookie and JSON body.
//  2. Browser JS reads the cookie (or body) and echoes it in X-CSRF-Token on mutations.
//  3. Middleware compares cookie vs header with constant-time equality.
//
// Safe methods (GET/HEAD/OPTIONS) are never blocked. Paths outside /api/v1/admin
// are ignored so public comment endpoints stay unaffected.
type CSRF struct {
	secureCookies bool
	// tokenTTL is only used for MaxAge on the cookie; validation is equality-based.
	tokenTTL time.Duration
	mu       sync.Mutex
}

func NewCSRF(secureCookies bool) *CSRF {
	return &CSRF{
		secureCookies: secureCookies,
		tokenTTL:      12 * time.Hour,
	}
}

func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAdminAPIPath(r.URL.Path) || isSafeMethod(r.Method) || isCSRFExempt(r) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			writeCSRFError(w)
			return
		}
		header := r.Header.Get(csrfHeaderName)
		if header == "" {
			// Also accept form field for non-JSON clients.
			_ = r.ParseForm()
			header = r.FormValue(csrfFormField)
		}
		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeCSRFError(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IssueTokenHandler sets a fresh CSRF cookie and returns the token in JSON.
func (c *CSRF) IssueTokenHandler(w http.ResponseWriter, r *http.Request) {
	token, err := randomToken(32)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(model.ErrorResponse{
			Error: model.ErrorDetail{Code: "INTERNAL_ERROR", Message: "Failed to issue CSRF token"},
		})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/api/v1/admin",
		HttpOnly: false, // must be readable by admin UI JS
		Secure:   c.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(c.tokenTTL.Seconds()),
		Expires:  time.Now().Add(c.tokenTTL),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"csrf_token": token})
}

func isAdminAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/admin")
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// Login must remain reachable without a prior CSRF token (bootstrapping).
func isCSRFExempt(r *http.Request) bool {
	return r.URL.Path == "/api/v1/admin/login" && r.Method == http.MethodPost
}

func writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    "CSRF_INVALID",
			Message: "Missing or invalid CSRF token",
		},
	})
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
