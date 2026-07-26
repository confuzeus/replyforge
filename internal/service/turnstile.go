package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	success   bool
	timestamp time.Time
}

type TurnstileVerifier struct {
	secretKey  string
	httpClient *http.Client
	cache      map[string]cacheEntry
	mu         sync.RWMutex
}

type turnstileResponse struct {
	Success bool `json:"success"`
}

func NewTurnstileVerifier(secretKey string) *TurnstileVerifier {
	return &TurnstileVerifier{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]cacheEntry),
	}
}

func (v *TurnstileVerifier) cacheKey(token, remoteIP string) string {
	hash := sha256.Sum256([]byte(token + ":" + remoteIP))
	return fmt.Sprintf("%x", hash)
}

func (v *TurnstileVerifier) Verify(ctx context.Context, answer, clientIP string) (bool, error) {

	key := v.cacheKey(answer, clientIP)

	v.mu.RLock()
	if entry, ok := v.cache[key]; ok && time.Since(entry.timestamp) < 5*time.Minute {
		v.mu.RUnlock()
		return entry.success, nil
	}
	v.mu.RUnlock()

	form := url.Values{}
	form.Set("secret", v.secretKey)
	form.Set("response", answer)
	form.Set("remoteip", clientIP)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("creating turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("turnstile request failed: %w", err)
	}
	defer resp.Body.Close()

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decoding turnstile response: %w", err)
	}

	v.mu.Lock()
	v.cache[key] = cacheEntry{success: result.Success, timestamp: time.Now()}
	v.mu.Unlock()

	return result.Success, nil
}
