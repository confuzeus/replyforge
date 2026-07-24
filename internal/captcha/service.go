package captcha

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

type ChallengeType = string

type GenerateOptions struct {
	Woodall string
	Rounds  int
}

type CaptchaStorage interface {
	Save(key, value string) bool
	Get(key string) (string, bool)
	Remove(key string) bool
}

type challengeEntry struct {
	Challenge string
	ExpiresAt time.Time
}

type InMemoryStorage struct {
	entries map[string]challengeEntry
	ttl     time.Duration
	mu      sync.RWMutex
	stop    chan struct{}
	stopped chan struct{}
}

func NewInMemoryStorage(ttl time.Duration) *InMemoryStorage {
	s := &InMemoryStorage{
		entries: make(map[string]challengeEntry),
		ttl:     ttl,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *InMemoryStorage) Stop() {
	close(s.stop)
	<-s.stopped
}

func (s *InMemoryStorage) cleanupLoop() {
	defer close(s.stopped)
	ticker := time.NewTicker(s.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *InMemoryStorage) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.ExpiresAt) {
			delete(s.entries, key)
		}
	}
}

func (s *InMemoryStorage) Save(key, value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = challengeEntry{
		Challenge: value,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	return true
}

func (s *InMemoryStorage) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return "", false
	}
	return entry.Challenge, true
}

func (s *InMemoryStorage) Remove(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.entries[key]
	delete(s.entries, key)
	return ok
}

type CaptchaService struct {
	storage CaptchaStorage
}

func NewCaptchaService(storage CaptchaStorage) *CaptchaService {
	return &CaptchaService{storage: storage}
}

func (s *CaptchaService) Stop() {
	if ims, ok := s.storage.(*InMemoryStorage); ok {
		ims.Stop()
	}
}

func (s *CaptchaService) GenerateChallenge(opts GenerateOptions) (string, string, error) {
	woodall, ok := resolveWoodall(opts.Woodall)
	if !ok {
		return "", "", fmt.Errorf("captcha: unknown woodall prime: %s", opts.Woodall)
	}

	p := Woodalls[woodall]
	if p == nil {
		return "", "", fmt.Errorf("captcha: woodall prime not found: %s", woodall)
	}

	one := big.NewInt(1)
	ns := make([]string, opts.Rounds)
	for i := 0; i < opts.Rounds; i++ {
		pMinusOne := new(big.Int).Sub(p, one)
		x, err := randomBigInt(pMinusOne)
		if err != nil {
			return "", "", fmt.Errorf("captcha: generating random: %w", err)
		}
		x = x.Add(x, one)
		n := new(big.Int).Mod(new(big.Int).Mul(x, x), p)
		ns[i] = BigIntToBase64(n)
	}

	inner := woodall + "," + strings.Join(ns, ",")
	encoded := base64.StdEncoding.EncodeToString([]byte(inner))
	challenge := "QuadraticResidueProblem," + encoded

	id := fmt.Sprintf("%x", md5.Sum([]byte(challenge)))
	s.storage.Save(id, challenge)

	return id, challenge, nil
}

func (s *CaptchaService) Verify(ctx context.Context, captchaID, answer, clientIP string) (bool, error) {
	stored, ok := s.storage.Get(captchaID)
	if !ok {
		return false, nil
	}
	s.storage.Remove(captchaID)

	parts := strings.SplitN(stored, ",", 2)
	if len(parts) != 2 || parts[0] != "QuadraticResidueProblem" {
		return false, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, nil
	}

	inner := strings.SplitN(string(decoded), ",", 2)
	if len(inner) != 2 {
		return false, nil
	}

	woodall, ok := resolveWoodall(inner[0])
	if !ok {
		return false, nil
	}

	p := Woodalls[woodall]
	if p == nil {
		return false, nil
	}

	challengeStrs := strings.Split(inner[1], ",")
	answers := strings.Split(answer, ",")
	if len(answers) != len(challengeStrs) {
		return false, nil
	}

	for i := range challengeStrs {
		expectedN, err := Base64ToBigInt(challengeStrs[i])
		if err != nil {
			return false, nil
		}
		ans, err := Base64ToBigInt(answers[i])
		if err != nil {
			return false, nil
		}
		resultN := new(big.Int).Mod(new(big.Int).Mul(ans, ans), p)
		if expectedN.Cmp(resultN) != 0 {
			return false, nil
		}
	}

	return true, nil
}

func resolveWoodall(aliasOrName string) (string, bool) {
	if _, ok := Woodalls[aliasOrName]; ok {
		return aliasOrName, true
	}
	if resolved, ok := WoodallAliases[aliasOrName]; ok {
		return resolved, true
	}
	return "", false
}

func randomBigInt(max *big.Int) (*big.Int, error) {
	bits := max.BitLen()
	bytes := (bits + 7) / 8
	buf := make([]byte, bytes)
	for {
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		result := new(big.Int).SetBytes(buf)
		mask := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		mask = mask.Sub(mask, big.NewInt(1))
		result = result.And(result, mask)
		if result.Cmp(max) < 0 {
			return result, nil
		}
	}
}
