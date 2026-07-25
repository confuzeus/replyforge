package captcha

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func buildChallenge(woodall string, answers []*big.Int) string {
	encValues := make([]string, len(answers))
	p := Woodalls[woodall]
	for i, x := range answers {
		n := new(big.Int).Mod(new(big.Int).Mul(x, x), p)
		encValues[i] = BigIntToBase64(n)
	}
	inner := woodall + "," + strings.Join(encValues, ",")
	encoded := base64.StdEncoding.EncodeToString([]byte(inner))
	return "QuadraticResidueProblem," + encoded
}

func buildAnswer(xs []*big.Int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = BigIntToBase64(x)
	}
	return strings.Join(parts, ",")
}

func TestVerify_Success(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	x1, _ := new(big.Int).SetString("42", 10)
	x2, _ := new(big.Int).SetString("100", 10)
	xs := []*big.Int{x1, x2}

	challenge := buildChallenge("751*2^751-1", xs)
	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	storage.Save(id, challenge)

	answer := buildAnswer(xs)
	ok, err := svc.Verify(nil, id, answer, "")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerify_WrongAnswer(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	x, _ := new(big.Int).SetString("42", 10)
	challenge := buildChallenge("751*2^751-1", []*big.Int{x})
	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	storage.Save(id, challenge)

	wrongX := new(big.Int).SetInt64(999)
	wrongAnswer := buildAnswer([]*big.Int{wrongX})
	ok, err := svc.Verify(nil, id, wrongAnswer, "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerify_ExpiredChallenge(t *testing.T) {
	storage := NewInMemoryStorage(1 * time.Nanosecond)
	svc := NewCaptchaService(storage, testLogger)

	x, _ := new(big.Int).SetString("42", 10)
	challenge := buildChallenge("751*2^751-1", []*big.Int{x})
	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	storage.Save(id, challenge)

	time.Sleep(10 * time.Millisecond)

	answer := buildAnswer([]*big.Int{x})
	ok, err := svc.Verify(nil, id, answer, "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerify_UnknownID(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	ok, err := svc.Verify(nil, "nonexistent", "any", "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerify_WrongRounds(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	x1, _ := new(big.Int).SetString("42", 10)
	x2, _ := new(big.Int).SetString("100", 10)
	challenge := buildChallenge("751*2^751-1", []*big.Int{x1, x2})
	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	storage.Save(id, challenge)

	answer := buildAnswer([]*big.Int{x1})
	ok, err := svc.Verify(nil, id, answer, "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerify_OneTimeUse(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	x, _ := new(big.Int).SetString("42", 10)
	challenge := buildChallenge("751*2^751-1", []*big.Int{x})
	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	storage.Save(id, challenge)

	answer := buildAnswer([]*big.Int{x})
	ok, err := svc.Verify(nil, id, answer, "")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.Verify(nil, id, answer, "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGenerateChallenge_ProducesValidOutput(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	id, challenge, err := svc.GenerateChallenge(GenerateOptions{Woodall: "2xs", Rounds: 2})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Contains(t, challenge, "QuadraticResidueProblem,")

	_, ok := storage.Get(id)
	assert.True(t, ok)
}

func TestGenerateChallenge_UnknownWoodall(t *testing.T) {
	svc := NewCaptchaService(NewInMemoryStorage(5 * time.Minute), testLogger)

	_, _, err := svc.GenerateChallenge(GenerateOptions{Woodall: "nonexistent", Rounds: 2})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown woodall prime")
}

func TestGenerateChallenge_AliasResolution(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	svc := NewCaptchaService(storage, testLogger)

	id, _, err := svc.GenerateChallenge(GenerateOptions{Woodall: "2xs", Rounds: 1})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

func TestBigIntToBase64_Roundtrip(t *testing.T) {
	tests := []string{
		"42",
		"1",
		"255",
		"1000000",
		"12345678901234567890",
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			n, ok := new(big.Int).SetString(s, 10)
			require.True(t, ok)
			encoded := BigIntToBase64(n)
			decoded, err := Base64ToBigInt(encoded)
			require.NoError(t, err)
			assert.Equal(t, n, decoded)
		})
	}
}

func TestBase64ToBigInt_Empty(t *testing.T) {
	_, err := Base64ToBigInt("")
	assert.Error(t, err)
}

func TestBase64ToBigInt_Invalid(t *testing.T) {
	_, err := Base64ToBigInt("!!!invalid!!!")
	assert.Error(t, err)
}

func TestInMemoryStorage_RemoveMissing(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	assert.False(t, storage.Remove("nonexistent"))
}

func TestInMemoryStorage_GetMissing(t *testing.T) {
	storage := NewInMemoryStorage(5 * time.Minute)
	_, ok := storage.Get("nonexistent")
	assert.False(t, ok)
}

func TestRandomBigInt_ProducesDifferentValues(t *testing.T) {
	max := new(big.Int).SetInt64(1 << 30)
	results := make(map[string]bool)
	for i := 0; i < 20; i++ {
		n, err := randomBigInt(max)
		require.NoError(t, err)
		assert.True(t, n.Cmp(max) < 0)
		assert.True(t, n.Cmp(big.NewInt(0)) >= 0)
		results[n.String()] = true
	}
	assert.Greater(t, len(results), 1)
}

func TestResolveWoodall(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		ok       bool
	}{
		{"md", "9531*2^9531-1", true},
		{"2xs", "751*2^751-1", true},
		{"3xl", "22971*2^22971-1", true},
		{"9531*2^9531-1", "9531*2^9531-1", true},
		{"nonexistent", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := resolveWoodall(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
