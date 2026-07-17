# Security Architecture

## Input Sanitization Pipeline

```go
package sanitizer

import (
    "strings"
    "golang.org/x/text/unicode/norm"
    "github.com/microcosm-cc/bluemonday"
)

type Sanitizer struct {
    htmlPolicy *bluemonday.Policy
}

func NewSanitizer() *Sanitizer {
    return &Sanitizer{
        htmlPolicy: bluemonday.StrictPolicy(), // Strips all HTML
    }
}

func (s *Sanitizer) Sanitize(input string) string {
    // 1. Strip all HTML tags and attributes
    sanitized := s.htmlPolicy.Sanitize(input)

    // 2. Trim leading/trailing whitespace
    sanitized = strings.TrimSpace(sanitized)

    // 3. Normalize Unicode to NFC form (prevents homoglyph attacks)
    sanitized = norm.NFC.String(sanitized)

    // 4. Collapse multiple consecutive spaces into single spaces
    sanitized = collapseSpaces(sanitized)

    // 5. Remove any null bytes
    sanitized = strings.ReplaceAll(sanitized, "\x00", "")

    return sanitized
}

func collapseSpaces(s string) string {
    return strings.Join(strings.Fields(s), " ")
}
```

## Rate Limiting Strategy

```go
package middleware

import (
    "net/http"
    "sync"
    "time"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    visitors map[string]*visitor
    mu       sync.Mutex
    rate     rate.Limit
    burst    int
}

type visitor struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

func NewRateLimiter(requestsPerMinute int, burstSize int) *RateLimiter {
    return &RateLimiter{
        visitors: make(map[string]*visitor),
        rate:     rate.Limit(requestsPerMinute / 60.0),
        burst:    burstSize,
    }
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := extractClientIP(r)

        limiter := rl.getVisitor(ip)
        if !limiter.Allow() {
            w.Header().Set("Retry-After", "60")
            w.WriteHeader(http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Configuration:**

- Default: 10 requests per minute per IP
- Burst: 15 requests (allows brief spikes)
- Cleanup: Background goroutine removes stale visitors every 5 minutes
- Only applies to POST /api/v1/comments endpoint

## Cloudflare Turnstile Integration

```go
package service

import (
    "context"
    "encoding/json"
    "net/http"
    "net/url"
    "time"
)

type TurnstileVerifier struct {
    secretKey  string
    httpClient *http.Client
    cache      map[string]cacheEntry // token_hash -> result
    cacheTTL   time.Duration
}

type TurnstileRequest struct {
    Secret   string `json:"secret"`
    Response string `json:"response"`
    RemoteIP string `json:"remoteip,omitempty"`
}

type TurnstileResponse struct {
    Success     bool     `json:"success"`
    ChallengeTS string   `json:"challenge_ts"`
    Hostname    string   `json:"hostname"`
    ErrorCodes  []string `json:"error-codes"`
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token string, remoteIP string) (bool, error) {
    // Check cache first (keyed by token hash + IP)
    cacheKey := hashTokenWithIP(token, remoteIP)
    if result, found := v.cache[cacheKey]; found {
        if time.Since(result.timestamp) < v.cacheTTL {
            return result.success, nil
        }
    }

    // Verify with Cloudflare
    formData := url.Values{
        "secret":   {v.secretKey},
        "response": {token},
        "remoteip": {remoteIP},
    }

    resp, err := v.httpClient.PostForm(
        "https://challenges.cloudflare.com/turnstile/v0/siteverify",
        formData,
    )
    if err != nil {
        return false, fmt.Errorf("turnstile request failed: %w", err)
    }
    defer resp.Body.Close()

    var result TurnstileResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, fmt.Errorf("turnstile response decode failed: %w", err)
    }

    // Cache the result
    v.cache[cacheKey] = cacheEntry{
        success:   result.Success,
        timestamp: time.Now(),
    }

    return result.Success, nil
}
```

**Verification Flow:**

1. Hash the token with the client IP (cache key)
2. Check in-memory cache (5-minute TTL)
3. If cache miss, send POST to Cloudflare Turnstile verify endpoint
4. Cache successful and failed results (prevents retry abuse)
5. Return verification result

## CORS Configuration

```go
package middleware

import (
    "net/http"
    "strings"
)

type CORSConfig struct {
    AllowedOrigins []string
    AllowedMethods []string
    AllowedHeaders []string
    MaxAge         int
}

func NewCORSConfig(origins string) *CORSConfig {
    return &CORSConfig{
        AllowedOrigins: strings.Split(origins, ","),
        AllowedMethods: []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type", "Accept"},
        MaxAge:         86400, // 24 hours
    }
}

func (c *CORSConfig) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")

        // Check if origin is allowed
        if c.isOriginAllowed(origin) {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.AllowedMethods, ", "))
            w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.AllowedHeaders, ", "))
            w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", c.MaxAge))

            // Handle preflight
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusNoContent)
                return
            }
        }

        next.ServeHTTP(w, r)
    })
}
```

**Configuration via Environment:**

```text
ALLOWED_ORIGINS=https://exampleblog.com,https://dev.exampleblog.com
```

### Data Protection Rules

| Data                     | Stored | Logged       | Returned in API |
| ------------------------ | ------ | ------------ | --------------- |
| Comment body (sanitized) | Yes    | No           | Yes             |
| Author name (sanitized)  | Yes    | No           | Yes             |
| Post ID                  | Yes    | Yes          | Yes             |
| IP address               | Yes    | Yes (hashed) | No              |
| User agent               | Yes    | No           | No              |
| Turnstile token          | No     | No           | No              |
| Display ID               | Yes    | Yes          | Yes             |
| Internal ID              | Yes    | Yes          | Yes             |

**Never log:** Turnstile tokens, raw request bodies, full IP addresses in plaintext

---
