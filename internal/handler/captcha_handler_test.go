package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/confuzeus/replyforge/internal/captcha"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaHandler_GenerateChallenge(t *testing.T) {
	storage := captcha.NewInMemoryStorage(5 * time.Minute)
	svc := captcha.NewCaptchaService(storage)

	handler := NewCaptchaHandler(CaptchaHandlerDependencies{
		Service:        svc,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		DefaultWoodall: "2xs",
		DefaultRounds:  2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge", nil)
	rec := httptest.NewRecorder()
	handler.GenerateChallenge(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ChallengeResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp.ID)
	assert.Contains(t, resp.Challenge, "QuadraticResidueProblem,")
}

func TestCaptchaHandler_GenerateChallenge_CustomParams(t *testing.T) {
	storage := captcha.NewInMemoryStorage(5 * time.Minute)
	svc := captcha.NewCaptchaService(storage)

	handler := NewCaptchaHandler(CaptchaHandlerDependencies{
		Service:        svc,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		DefaultWoodall: "md",
		DefaultRounds:  2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge?woodall=2xs&rounds=3", nil)
	rec := httptest.NewRecorder()
	handler.GenerateChallenge(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ChallengeResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp.ID)
}

func TestCaptchaHandler_GenerateChallenge_InvalidWoodall(t *testing.T) {
	storage := captcha.NewInMemoryStorage(5 * time.Minute)
	svc := captcha.NewCaptchaService(storage)

	handler := NewCaptchaHandler(CaptchaHandlerDependencies{
		Service:        svc,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		DefaultWoodall: "md",
		DefaultRounds:  2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge?woodall=nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.GenerateChallenge(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCaptchaHandler_GenerateChallenge_InvalidRounds(t *testing.T) {
	storage := captcha.NewInMemoryStorage(5 * time.Minute)
	svc := captcha.NewCaptchaService(storage)

	handler := NewCaptchaHandler(CaptchaHandlerDependencies{
		Service:        svc,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		DefaultWoodall: "2xs",
		DefaultRounds:  2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge?rounds=invalid", nil)
	rec := httptest.NewRecorder()
	handler.GenerateChallenge(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
