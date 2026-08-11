---
name: csrf-protection
description: How to implement and use CSRF (cross-site request forgery) protection for cookie-authenticated, state-changing HTTP endpoints in Go 1.25+ using net/http.CrossOriginProtection (Sec-Fetch-Site and Origin based detection). Use when adding, configuring, or testing CSRF defenses for admin or state-changing API routes.
---

# CSRF Protection with `net/http.CrossOriginProtection`

Protects cookie-authenticated, state-changing HTTP endpoints against Cross-Site Request Forgery using Go 1.25+'s standard library. No external dependencies.

---

## 1. Threat Model

Browsers automatically attach cookies to requests, including to cross-origin ones. If a victim is logged into your site, an attacker can trigger state-changing requests from an attacker-controlled page (e.g. via a form POST or an `img`/`fetch`-based delete) and the browser will include the victim's session cookie. CSRF protection blocks these cross-origin requests.

This mechanism is designed for **cookie-authenticated APIs**. Non-browser clients (CLI tools, curl, service accounts) authenticate with the same cookies but do not send browser fetch metadata, so they must keep working — see §3.

---

## 2. Prerequisites

- **Go 1.25 or newer** (`http.CrossOriginProtection` was added in Go 1.25).
- No third-party packages.

---

## 3. How the Checks Work

`http.NewCrossOriginProtection()` returns a value whose `Check(*http.Request) error` reports cross-origin browser requests. It currently detects them via:

1. **`Sec-Fetch-Site` header** (primary) — sent by all modern browsers (since ~2023). A value of `cross-site` indicates the request did not come from your origin.
2. **`Origin` vs `Host` fallback** — for browsers that omit `Sec-Fetch-Site`, the hostname of the `Origin` header is compared to the request's `Host`. A mismatch is rejected.

Rules the stdlib enforces:

- **Safe methods** — `GET`, `HEAD`, `OPTIONS` — are **always allowed**. Your application must therefore perform **no state changes** in response to safe methods.
- **Requests with neither `Sec-Fetch-Site` nor `Origin`** are assumed to be same-origin or non-browser and are **allowed**. This is what keeps curl/CLI clients working — and why this approach cannot stop a non-browser client that already holds a session cookie.
- The **zero value of `CrossOriginProtection` is valid** and denies cross-origin requests out of the box.

---

## 4. Implementation

Build a small middleware that scopes the check to the routes you want to protect (a configurable path prefix + non-safe methods). Wrap it around the whole mux, or only the protected sub-router.

```go
package middleware

import (
	"net/http"
	"strings"
)

// CSRF protects state-changing requests to paths under prefix against
// cross-origin browser attacks using http.CrossOriginProtection.
type CSRF struct {
	cop    *http.CrossOriginProtection
	prefix string // only paths under this prefix are checked
}

// NewCSRF returns a CSRF middleware that protects paths under prefix.
func NewCSRF(prefix string) *CSRF {
	return &CSRF{
		cop:    http.NewCrossOriginProtection(),
		prefix: prefix,
	}
}

func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, c.prefix) || isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.cop.Check(r); err != nil {
			writeCSRFDenied(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func writeCSRFDenied(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	// Encode an error body consistent with the rest of your API, e.g.:
	// {"error": {"code": "CSRF_INVALID", "message": "Cross-origin request denied"}}
}
```

### Wiring

- Construct the middleware once and apply it in the middleware chain **inside** the auth and CORS layers — order relative to those does not matter for the check itself, but the CSRF response should be a 403, not a 401.
- Typical chain (outer → inner): `Recovery → Logging → CORS → Auth → CSRF → Handler`. Or apply CSRF + Auth per-subrouter if your framework supports it.
- Only **state-changing** routes under the prefix need protection. Safe-method routes (list/detail/fetch) are skipped automatically, but ensure they never mutate state.
- Public, unauthenticated endpoints outside the prefix are unaffected.

---

## 5. Configuration & Extension

`CrossOriginProtection` exposes an additional surface beyond `Check`:

| Method | Purpose |
| --- | --- |
| `AddTrustedOrigin(origin string) error` | Allow a specific cross-origin origin to pass the check. |
| `AddInsecureBypassPattern(pattern string)` | Skip the check for requests matching a regexp path pattern (e.g. a webhook endpoint). Use sparingly and only for non-cookie-authenticated routes. |
| `SetDenyHandler(h Handler)` | Customize the response when a request is denied (default is `403 Forbidden`). |
| `Handler(h Handler)` | Convenience wrapper: returns an `http.Handler` that runs `Check` for **all** paths, not just non-safe ones. Use the middleware form in §4 when you need method/path scoping. |

---

## 6. Defense in Depth

The middleware is only one layer. Harden the session cookie too:

```go
http.SetCookie(w, &http.Cookie{
	Name:     "session",
	Value:    token,
	Path:     "/api/admin",        // narrow scope: only sent to protected routes
	HttpOnly: true,                // inaccessible to JavaScript (XSS cannot read it)
	Secure:   true,                // only over HTTPS
	SameSite: http.SameSiteStrictMode, // browsers refuse to attach it cross-site
	MaxAge:   int(ttl.Seconds()),
})
```

- `SameSite=Strict` alone blocks most modern-browser CSRF even without this middleware; treat it as a complement, not a replacement.
- Keep CORS responses restrictive — if you reflect arbitrary origins, cross-origin JavaScript could read responses. CSRF defends the *requests*; CORS defends the *responses*.
- Do not make safe methods (GET/HEAD/OPTIONS) perform writes — the middleware explicitly allows them.

---

## 7. Testing

Cover the full behavior matrix with table-driven tests using `net/http/httptest`. Required cases:

| Case | Setup | Expect |
| --- | --- | --- |
| Safe method allowed | `GET` on protected path | pass |
| Same-origin mutation allowed | `POST` protected path, `Sec-Fetch-Site: same-origin` | pass |
| Cross-site mutation blocked | `POST` protected path, `Sec-Fetch-Site: cross-site` | `403` + error code |
| Origin-mismatch blocked | `POST` protected path, `Origin: https://evil.example.com`, `Host: app.example.com` | `403` |
| Non-browser allowed | `DELETE` protected path, no `Sec-Fetch-Site`/`Origin` headers | pass |
| Out-of-scope path unaffected | `POST` to a path outside the prefix with `Sec-Fetch-Site: cross-site` | pass |

Example:

```go
func TestCSRF_BlocksCrossSiteMutation(t *testing.T) {
	csrf := NewCSRF("/api/admin")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := csrf.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/items/1/delete", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
```

Also assert the JSON error body shape so API consumers get a stable contract.

---

## 8. Limitations & When to Choose Another Approach

- **Not a defense against non-browser clients** that hold a session cookie (e.g. exfiltrated via XSS). Pair with `HttpOnly` cookies and XSS defenses.
- **Relies on an accurate `Host` header** for the Origin-fallback path; be wary of host-header injection if you must configure `AddTrustedOrigin`.
- **Some older clients omit `Sec-Fetch-Site`**, falling back to the weaker Origin comparison.
- Safe-method endpoints are a potential gap if a developer later adds state changes to a GET. Guard this in review.

If you need stronger guarantees (e.g. the client is untrusted JavaScript, or you want protection that works even against clients that can read cookies), consider a **synchronizer token** (server-rendered token field verified on each state change) or a **double-submit cookie** scheme instead.
