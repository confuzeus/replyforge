package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimit_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(60, 5)
	defer rl.Stop()

	handler := rl.Middleware(okHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should pass", i+1)
	}
}

func TestRateLimit_OverLimit(t *testing.T) {
	rl := NewRateLimiter(60, 3)
	defer rl.Stop()

	handler := rl.Middleware(okHandler())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should pass", i+1)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	defer rl.Stop()

	handler := rl.Middleware(okHandler())

	reqA := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	reqA.Header.Set("X-Forwarded-For", "10.0.0.1")
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	reqB.Header.Set("X-Forwarded-For", "10.0.0.2")

	for i := 0; i < 2; i++ {
		recA := httptest.NewRecorder()
		handler.ServeHTTP(recA, reqA)
		assert.Equal(t, http.StatusOK, recA.Code)

		recB := httptest.NewRecorder()
		handler.ServeHTTP(recB, reqB)
		assert.Equal(t, http.StatusOK, recB.Code)
	}

	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	assert.Equal(t, http.StatusTooManyRequests, recA.Code)

	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)
	assert.Equal(t, http.StatusTooManyRequests, recB.Code)
}

func TestRateLimit_Cleanup(t *testing.T) {
	rl := NewRateLimiter(60, 1)
	defer rl.Stop()

	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.99")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	rl.mu.Lock()
	assert.Contains(t, rl.visitors, "10.0.0.99")
	rl.visitors["10.0.0.99"].lastSeen = time.Now().Add(-10 * time.Minute)
	rl.mu.Unlock()

	rl.mu.Lock()
	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > 5*time.Minute {
			delete(rl.visitors, ip)
		}
	}
	rl.mu.Unlock()

	rl.mu.Lock()
	assert.NotContains(t, rl.visitors, "10.0.0.99")
	rl.mu.Unlock()
}

func TestExtractClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	assert.Equal(t, "10.0.0.1", ExtractClientIP(req))
}

func TestExtractClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.5")

	assert.Equal(t, "10.0.0.5", ExtractClientIP(req))
}

func TestExtractClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	assert.Equal(t, "192.168.1.1", ExtractClientIP(req))
}

func TestStop_ClosesChannel(t *testing.T) {
	rl := NewRateLimiter(60, 3)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-rl.stopCh:
		case <-time.After(1 * time.Second):
			t.Error("stopCh was not closed within timeout")
		}
	}()

	rl.Stop()
	wg.Wait()
}
