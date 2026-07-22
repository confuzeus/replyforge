package middleware

import "net/http"

type CORSConfig struct {
	allowedOrigins []string
}

func NewCORSConfig(origins []string) *CORSConfig {
	return &CORSConfig{allowedOrigins: origins}
}

func (c *CORSConfig) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !c.isAllowed(origin) {
			next.ServeHTTP(w, r)
			return
		}

		allowOrigin := origin
		if len(c.allowedOrigins) == 1 && c.allowedOrigins[0] == "*" {
			allowOrigin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CORS(cfg *CORSConfig) func(http.Handler) http.Handler {
	return cfg.Middleware
}

func (c *CORSConfig) isAllowed(origin string) bool {
	for _, allowed := range c.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
