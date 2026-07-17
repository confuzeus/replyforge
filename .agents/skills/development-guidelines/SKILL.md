---
name: development-guidelines
description: "Go Project Development Guidelines. Use when writing new code or refactoring existing code."
---

# Go Project Development Guidelines

This document captures a pragmatic, experience-backed approach to writing Go for a project that starts small but is built to last. It targets a single maintainer today while leaving the door open for a team tomorrow. The overriding philosophy: **keep it simple, keep it idiomatic, and only add complexity when the code asks for it.**

---

## 1. Project Structure – Start Flat, Grow on Demand

Go prefers convention over ceremony. Avoid elaborate directory hierarchies before they are justified.

**Rules of thumb:**

- **`cmd/`** – One directory per executable. Keep `main.go` thin – parse config, wire dependencies, start the server/listener.
- **`internal/`** – Everything that shouldn’t be imported by external projects. Structure by **domain module**, not by technical layer. A “user” package may contain its own handler, service, and store when the package remains small; extract sub-packages when it grows uncomfortable.
- **`pkg/`** – Use sparingly. First place code inside `internal/`, extract to `pkg/` only when you genuinely need to share logic across multiple binaries within the same module.
- **Avoid** `models/`, `utils/`, `helpers/`, `common/` packages. They become dumping grounds. Prefer descriptive, domain-centric package names (`user`, `invoice`, `policy`).

_Scaling hint:_ When a package becomes large, split it vertically by feature (e.g., `internal/user` → `internal/user/service`, `internal/user/store`). This mirrors how you’d later extract a microservice without rewriting.

---

## 2. Coding Conventions – Write Clear Go, Nothing Fancy

### Formatting & Linting

- Always use `gofmt` (or `goimports`) on save.
- Run `go vet` and `staticcheck` as part of your edit-save cycle.
- Configure `golangci-lint` with a moderate set of linters (see §8). Do not disable linters just to silence them; understand why they fire and fix the root cause.

### Naming

- **Clarity over brevity.** Names carry meaning across boundaries. A short name like `ctx` is fine locally; a function `Process` is not.
- **No stutter.** `user.UserStore` → `user.Store`. Package name already provides context.
- **Use mixedCaps**, not underscores (except for test files: `store_test.go`).
- **Exported identifiers** only when needed. Start unexported and promote when a real consumer appears.

### Interfaces

- **Define interfaces where they are used** (consumer side), not where they are implemented.
- Keep interfaces **small** (1–3 methods). Large interfaces are a design smell. The standard library’s `io.Reader` is the model.
- Do not create interfaces “just in case.” Return concrete types from functions; accept interfaces in function parameters only when you need to swap behaviour for testing or multiple implementations.

### Functions & Control Flow

- **Early returns** keep the happy path flat. Avoid deep nesting.
- Functions should do **one thing**. If you need a bulleted list to explain what a function does, break it up.
- Use **value receivers** by default; switch to pointer receivers when you need to mutate state or when the struct is large enough that copying becomes a measurable cost.
- **Avoid `init()`.** Explicit initialisation is easier to reason about and test. The main exception is registering database drivers or other truly static side-effects.

### Constants & Enums

- Use `iota` for typed constants.
- Prefer string-backed constants for serialisation stability, e.g.:
  ```go
  type Status string
  const (
      StatusActive   Status = "active"
      StatusInactive Status = "inactive"
  )
  ```

---

## 3. Error Handling – Never Ignore, Always Contextualise

```go
if err != nil {
    return fmt.Errorf("fetching user %s: %w", userID, err)
}
```

- **Wrap errors** with `fmt.Errorf` and the `%w` verb so callers can use `errors.Is` and `errors.As`.
- **Use sentinel errors** for fixed conditions that clients check for:
  ```go
  var ErrNotFound = errors.New("not found")
  ```
- **Use custom error types** only when you need to attach structured metadata. Keep them simple.
- **Do not panic** in library code. In `main`, a `panic` is acceptable only when the program literally cannot start (missing mandatory config).
- **Log once, at the top.** A function that returns an error should not also log it; let the caller decide. The highest level handler (HTTP middleware, CLI command) is usually the right place to log.

---

## 4. Testing – Make It Easy to Verify Correctness

### Unit Tests

- **Table-driven tests** are the Go norm. They read well and make adding cases trivial.
- Use `t.Run` for subtests to get parallel execution and clear failure messages.
- Prefer the standard library `testing` package. Add `testify/assert` if it reduces boilerplate, but never obscure test intent behind fluent chains.
- **Mock with interfaces.** If a function depends on an external system, accept an interface. Generate mocks with `mockgen` or write tiny manual mocks. Keep mocks in `*_test.go` files or a dedicated `mocks` package inside the test.

### Integration / Database Tests

- Test against a real database (run via Docker in CI). Use a helper to spin up a fresh instance or a transaction that rolls back.
- Seed the minimum necessary data per test. Tests should not depend on shared global state.

### Coverage

- Aim for **meaningful coverage** on business logic, not mechanical line counts. 80%+ on domain/service packages is a healthy target. Don’t obsess over 100%.

### Race Detector

Always run tests with `-race` in CI and locally before committing.

---

## 5. Concurrency – Communicate to Share

- **Goroutines are cheap; lifecycle management is not.** Always know how a goroutine will stop.
- **Use `context.Context`** for deadlines, cancellation, and request-scoped values. Pass it as the first argument of every function that crosses process boundaries.
- **Prefer `sync.WaitGroup` or `errgroup.Group`** to coordinate goroutines and collect errors. The `errgroup` package (`golang.org/x/sync/errgroup`) is invaluable for “run N tasks and fail on first error”.
- **Channels or mutexes?** Use whichever makes the data flow obvious. A buffered channel as a work queue is clear; a mutex protecting a map is also clear. Avoid mixing them in ways that create hidden coupling.
- **No shared mutable state without synchronisation.** Run your tests with the race detector to catch violations early.
- **Never start goroutines inside a library without exposing a shutdown or cancellation mechanism.**

---

## 6. Dependencies – Standard Library First

- **Minimise third-party dependencies.** Every import carries a maintenance burden and a trust decision.
- Start with the standard library. Add an external package only when it saves significant time or fills a well-defined gap (structured logging, configuration parsing, HTTP routing with path parameters).
- **Favour well-known, stable projects** with compatible licences (MIT, Apache 2.0). Keep them up to date with `go get -u` and `go mod tidy`.
- Pin direct dependencies to a **minimum version** and let `go.sum` lock them. Commit both `go.mod` and `go.sum`.

---

## 7. Observability – Structured Logging & Metrics (Optional Start)

- **Structured logging** from day one. The standard library’s `log/slog` (Go 1.21+) is the default choice. If you need richer features, `zerolog` or `zap` are acceptable, but `slog` covers 95% of cases.
- Use consistent log keys: `"user_id"`, `"request_id"`, `"error"`, `"duration_ms"`.
- **Do not log in library/package code.** Return errors and let the application layer decide how to record them.
- If you add metrics, use `expvar` for simple counters or `prometheus/client_golang`. Don’t add them before you have a dashboard that actually consumes them; the value is in the query, not the emission.

---

## 8. Tooling & Continuous Integration

### Local Environment

```makefile
# .PHONY targets for common operations
.PHONY: fmt lint test build

fmt:
	goimports -w .

lint:
	golangci-lint run

test:
	go test -race -count=1 ./...

build:
	go build -o bin/server ./cmd/server
```

Recommended `golangci-lint` configuration (`.golangci.yml`):

- Enable: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`, `bodyclose`, `misspell`, `gocritic`
- Add more only when you understand the signal they produce.

### CI (GitHub Actions / equivalent)

Pipeline should:

1. Check formatting (`gofmt -d .`)
2. Lint (`golangci-lint run`)
3. Test (`go test -race -shuffle=on ./...`)
4. Build binaries

Pre-commit hooks can run `gofmt` and `goimports`; let CI enforce the rest.

---

## 9. API / HTTP Best Practices (If Applicable)

- Use standard `net/http` with a lightweight router like `chi` when you need path parameters and middleware chaining. Avoid framework lock-in.
- Handlers should be thin: extract input, call a service, write output.
- Use `context.Context` for request-scoped values (tracing ID, authenticated user). Never put a context in a struct.
- Validate input explicitly. Use `go-playground/validator` only if you have many complex struct validations; a simple `if x == ""` is often clearer.
- Version your API from the start: `/api/v1/...`. It’s a cheap insurance policy.
- Encode/decode JSON with `encoding/json`. For large codebases, the performance of `jsoniter` may become worth it later; that’s a deliberate optimisation, not a default.

---

## 10. Scaling Considerations – Plan by Decoupling, Not by Over-Engineering

When the project needs to grow in complexity or team size, the guidelines above already provide natural seams:

- **Packages map to bounded contexts.** A clear domain package can become an independent service later.
- **Interfaces at package boundaries** allow swapping implementations (in-memory store → PostgreSQL → gRPC client) without cascading changes.
- **Thin handlers and service layers** mean you can expose the same logic over HTTP, gRPC, or a CLI without duplication.
- **Avoid repository pattern until you need it.** In the beginning, a concrete `store` type that directly talks to the database is fine. When you have multiple backends or need extensive test mocks, introduce the interface. The refactoring is mechanical and safe.
- **Don’t build a message queue until you need one.** Start with synchronous calls; extract asynchronous workflows when latency or reliability demands it.

The key: the code stays **simple to change** because every abstraction is introduced reactively, not speculatively.

---

## Summary Checklist

- [ ] Code committed matches `gofmt`/`goimports`.
- [ ] Passes `go vet` and configured linters.
- [ ] Tests pass with `-race`.
- [ ] Every exported identifier has a meaningful godoc comment.
- [ ] Errors are wrapped; no `panic` outside of `main`.
- [ ] Goroutines have clear lifetimes; context is propagated.
- [ ] Configuration is externalised (env vars or file), no hardcoded secrets.
- [ ] `go.mod` and `go.sum` are tidy and committed.
- [ ] `README.md` explains how to build, run, and test.
