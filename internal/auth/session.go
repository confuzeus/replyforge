package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/confuzeus/replyforge/internal/model"
)

type SessionManager struct {
	mu            sync.RWMutex
	sessions      map[string]*session
	ttl           time.Duration
	secureCookies bool
	stopCh        chan struct{}
}

type session struct {
	expiresAt time.Time
}

func NewSessionManager(ttl time.Duration, secureCookies bool) *SessionManager {
	sm := &SessionManager{
		sessions:      make(map[string]*session),
		ttl:           ttl,
		secureCookies: secureCookies,
		stopCh:        make(chan struct{}),
	}
	go sm.cleanupLoop()
	return sm
}

func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

func (sm *SessionManager) CreateSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	sm.mu.Lock()
	sm.sessions[token] = &session{
		expiresAt: time.Now().Add(sm.ttl),
	}
	sm.mu.Unlock()

	return token, nil
}

func (sm *SessionManager) DeleteSession(token string) {
	sm.mu.Lock()
	delete(sm.sessions, token)
	sm.mu.Unlock()
}

func (sm *SessionManager) ValidateSession(token string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(s.expiresAt) {
		delete(sm.sessions, token)
		return false
	}
	return true
}

func (sm *SessionManager) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/api/v1/admin",
		HttpOnly: true,
		Secure:   sm.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sm.ttl.Seconds()),
		Expires:  time.Now().Add(sm.ttl),
	})
}

func (sm *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/api/v1/admin",
		HttpOnly: true,
		Secure:   sm.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (sm *SessionManager) GetToken(r *http.Request) string {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (sm *SessionManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sm.GetToken(r)
		if token == "" || !sm.ValidateSession(token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(model.ErrorResponse{
				Error: model.ErrorDetail{
					Code:    "UNAUTHORIZED",
					Message: "Not authenticated",
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sm.mu.Lock()
			now := time.Now()
			for token, s := range sm.sessions {
				if now.After(s.expiresAt) {
					delete(sm.sessions, token)
				}
			}
			sm.mu.Unlock()
		case <-sm.stopCh:
			return
		}
	}
}
