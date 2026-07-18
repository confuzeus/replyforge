# AGENTS.md — Reply Forge

## Build & Run

- Task runner: **`just`**, not `make`. Commands: `just fmt`, `just lint`, `just test`, `just build`, `just run`.
- `just build` runs `go build -ldflags="-s -w" -o bin/server ./cmd/server`.
- **CGO_ENABLED=1 is required** for all builds (SQLite via `go-sqlite3` CGO driver).
- Version is set via the `VERSION` env var (defaults to `"dev"` in `config.Load()`).
- Single entrypoint: `cmd/server/main.go`.

## Tests

- Run: `go test -race -count=1 ./...` (or `just test`).
- Single package: `go test -race -count=1 ./internal/service/`.
- Tests use **in-memory SQLite** (`:memory:?_journal_mode=WAL&_foreign_keys=on`) with `config.RunMigrations(db, migrations.SQL)`. The `migrations.SQL` is an `embed.FS` of `*.sql` files.
- No mock framework — hand-written stubs (e.g., `stubTurnstileVerifier`). Uses `testify/require` + `testify/assert`.
- `setupTestDB(t)` and `seedApprovedComment(t, db, ...)` helpers are duplicated per package.

## Architecture

- No router library — uses Go 1.22+ `http.ServeMux` with method+path patterns (`GET /api/v1/comments/{id}`).
- Middleware order (outer→inner): **Recovery → Logging → CORS → RateLimit → Handler**.
- Layers: handler → service → repository → SQLite.

## Database & Migrations

- SQLite with `?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`.
- Migrations sorted alphabetically. **PRAGMA runs outside transaction**; DDL/DML runs inside tx+commit. Mixing them requires separate files.

## Quirks & Gotchas

- `GetByID` returns `NOT_FOUND` for both missing comments **and** comments with `approved=0`.
- All comments created with `approved=false` — no in-app approval UI.
- **Rate limiting only applies to `POST /api/v1/comments`.** Other endpoints un-throttled.
- Turnstile verification cached in-memory 5 min (key: SHA256(token+IP)).
- Client IPs are **SHA256-hashed in request logs** (PII protection in `middleware/logging.go`).
- Logging middleware **skips `/health` and `/debug/vars`** — no request_id, no log entry.
- `ServiceError` implements `Unwrap()` — use `errors.As` to extract from handler.
- Sanitizer chain: bluemonday StrictPolicy → trim → NFC normalize → collapse whitespace → null byte strip. Runs on `AuthorName` and `Body` in the service layer.
- `godotenv.Load()` called on startup but **silently ignores missing `.env`**.
- No `.golangci.yml` or CI workflow config exists — `golangci-lint` uses defaults.

## Conventions

- Format: `goimports -w .` (via `just fmt`), not plain `gofmt`.
- Config: env vars only (`internal/config/config.go`). See `.env.example`.
- Structured JSON logging via `log/slog`. Keys: `request_id`, `client_ip` (hashed), `error`, `duration_ms`.
- Do not log PII in general. Exceptions: `author_name` in the create-comment log line, and user-agent in the request log line.
- Use `just` for all dev workflows — the justfile is the source of truth for dev commands.
