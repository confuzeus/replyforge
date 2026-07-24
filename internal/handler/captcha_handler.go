package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/confuzeus/replyforge/internal/captcha"
)

type CaptchaHandler struct {
	service  *captcha.CaptchaService
	logger   *slog.Logger
	defaults capchaDefaults
}

type capchaDefaults struct {
	woodall string
	rounds  int
}

type CaptchaHandlerDependencies struct {
	Service        *captcha.CaptchaService
	Logger         *slog.Logger
	DefaultWoodall string
	DefaultRounds  int
}

func NewCaptchaHandler(deps CaptchaHandlerDependencies) *CaptchaHandler {
	return &CaptchaHandler{
		service: deps.Service,
		logger:  deps.Logger,
		defaults: capchaDefaults{
			woodall: deps.DefaultWoodall,
			rounds:  deps.DefaultRounds,
		},
	}
}

func (h *CaptchaHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/captcha/challenge", h.GenerateChallenge)
}

type ChallengeResponse struct {
	ID        string `json:"id"`
	Challenge string `json:"challenge"`
}

func (h *CaptchaHandler) GenerateChallenge(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	woodall := h.defaults.woodall
	if v := query.Get("woodall"); v != "" {
		woodall = v
	}

	rounds := h.defaults.rounds
	if v := query.Get("rounds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			rounds = n
		}
	}

	id, challenge, err := h.service.GenerateChallenge(captcha.GenerateOptions{
		Woodall: woodall,
		Rounds:  rounds,
	})
	if err != nil {
		h.logger.Error("failed to generate challenge", "error", err)
		writeError(w, http.StatusBadRequest, "CHALLENGE_GENERATION_FAILED", "Failed to generate captcha challenge", nil)
		return
	}

	h.logger.Info("captcha challenge generated",
		"challenge_id", id,
		"woodall", woodall,
		"rounds", rounds,
	)

	resp := ChallengeResponse{
		ID:        id,
		Challenge: challenge,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(resp)
}
