package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
	logger   *slog.Logger
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(requestsPerMinute, burst int, logger *slog.Logger) *RateLimiter {
	r := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(requestsPerMinute) / 60,
		burst:    burst,
		stopCh:   make(chan struct{}),
		logger:   logger,
	}
	go r.cleanupVisitors()
	return r
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCreateComment := r.Method == http.MethodPost && r.URL.Path == "/api/v1/comments"
		isAdminMutation := (r.Method == http.MethodPost || r.Method == http.MethodDelete) &&
			strings.HasPrefix(r.URL.Path, "/api/v1/admin/comments/")
		isAdminLogin := r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/login"

		if !isCreateComment && !isAdminMutation && !isAdminLogin {
			next.ServeHTTP(w, r)
			return
		}
		ip := ExtractClientIP(r)
		limiter := rl.getVisitor(ip)

		if !limiter.Allow() {
			RateLimitHitsTotal.Add(1)
			if rl.logger != nil {
				rl.logger.Warn("rate limit exceeded",
					"client_ip", ip,
					"path", r.URL.Path,
				)
			}
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"Too many requests"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 5*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return rl.Middleware
}

func ExtractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
