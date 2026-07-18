package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndVerify(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := GenerateHash(password)
	require.NoError(t, err)

	ok, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestWrongPassword(t *testing.T) {
	hash, err := GenerateHash("real-password")
	require.NoError(t, err)

	ok, err := VerifyPassword("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEmptyPassword(t *testing.T) {
	hash, err := GenerateHash("something")
	require.NoError(t, err)

	ok, err := VerifyPassword("", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestInvalidHashFormat(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"too few segments", "$argon2id$v=19$m=8,t=1,p=1$salt"},
		{"too many segments", "$a$b$c$d$e$f$g"},
		{"empty string", ""},
		{"wrong algorithm", "$argon2i$v=19$m=8,t=1,p=1$salt$hash"},
		{"invalid base64 salt", "$argon2id$v=19$m=8,t=1,p=1$!!!$hash"},
		{"invalid version", "$argon2id$v=99$m=8,t=1,p=1$c2FsdA==$hash"},
		{"missing params", "$argon2id$v=19$$c2FsdA==$aGFzaA=="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyPassword("password", tt.hash)
			assert.Error(t, err)
		})
	}
}

func TestHashConsistency(t *testing.T) {
	hash1, err := GenerateHash("same-password")
	require.NoError(t, err)

	hash2, err := GenerateHash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "hashes should differ due to random salt")

	ok1, err := VerifyPassword("same-password", hash1)
	require.NoError(t, err)
	assert.True(t, ok1)

	ok2, err := VerifyPassword("same-password", hash2)
	require.NoError(t, err)
	assert.True(t, ok2)
}

func TestHashFormat(t *testing.T) {
	hash, err := GenerateHash("test-password")
	require.NoError(t, err)

	assert.Contains(t, hash, "$argon2id$")
	assert.Contains(t, hash, "v=19")
	// PHC format has 6 $-separated segments: $alg$v=n$params$salt$hash
	assert.Equal(t, 5, strings.Count(hash, "$"), "PHC string should have 5 $ characters")
}
