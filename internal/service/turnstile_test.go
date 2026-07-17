package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheKey_Deterministic(t *testing.T) {
	v := NewTurnstileVerifier("secret")

	key1 := v.cacheKey("token-a", "1.2.3.4")
	key2 := v.cacheKey("token-a", "1.2.3.4")

	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 64) // SHA-256 hex = 64 chars
}

func TestCacheKey_DifferentInputs(t *testing.T) {
	v := NewTurnstileVerifier("secret")

	tests := []struct {
		name   string
		token1 string
		ip1    string
		token2 string
		ip2    string
	}{
		{"different token", "token-a", "1.2.3.4", "token-b", "1.2.3.4"},
		{"different IP", "token-a", "1.2.3.4", "token-a", "5.6.7.8"},
		{"both different", "token-a", "1.2.3.4", "token-b", "5.6.7.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := v.cacheKey(tt.token1, tt.ip1)
			key2 := v.cacheKey(tt.token2, tt.ip2)
			assert.NotEqual(t, key1, key2)
		})
	}
}

func TestCacheKey_EmptyInputs(t *testing.T) {
	v := NewTurnstileVerifier("secret")

	key := v.cacheKey("", "")
	assert.Len(t, key, 64)
	assert.NotEmpty(t, key)
}

func TestNewTurnstileVerifier_Defaults(t *testing.T) {
	v := NewTurnstileVerifier("my-secret-key")

	assert.Equal(t, "my-secret-key", v.secretKey)
	assert.NotNil(t, v.httpClient)
	assert.Equal(t, 10*time.Second, v.httpClient.Timeout)
	assert.NotNil(t, v.cache)
	assert.Empty(t, v.cache)
}
