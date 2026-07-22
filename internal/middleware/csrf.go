package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/confuzeus/replyforge/internal/model"
)

// CSRF protects state-changing admin requests against cross-origin browser attacks
// using Go 1.25's http.CrossOriginProtection.
//
// The middleware checks Sec-Fetch-Site and Origin headers on non-safe (POST, DELETE, etc.)
// requests to /api/v1/admin/*. Cross-origin browser requests are rejected with 403.
// Non-browser clients (which lack these headers) are allowed through, since they are
// not vulnerable to CSRF.
type CSRF struct {
	cop *http.CrossOriginProtection
}

func NewCSRF() *CSRF {
	return &CSRF{
		cop: http.NewCrossOriginProtection(),
	}
}

func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAdminAPIPath(r.URL.Path) || isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.cop.Check(r); err != nil {
			writeCSRFError(w)
			return
		}
		next.ServeHTTP(w, r)
	})
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

func writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    "CSRF_INVALID",
			Message: "Cross-origin request denied",
		},
	})
}
