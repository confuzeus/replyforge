# Reply Forge

A REST API service for managing blog comments with anti-spam protection via Cloudflare Turnstile. The system stores comments in SQLite and serves approved comments to readers while protecting against automated spam.

## Quick Start

```bash
cp .env.example .env
# Edit .env with your Turnstile secret key and desired config
just build && just run
```

## Configuration

Configuration is loaded from environment variables. Copy `.env.example` to `.env` and adjust as needed.

| Variable               | Default            | Description                                 |
| ---------------------- | ------------------ | ------------------------------------------- |
| `PORT`                 | `8080`             | Server listen port                          |
| `DATABASE_PATH`        | `data/comments.db` | SQLite database file path                   |
| `LOG_LEVEL`            | `info`             | Log level: `debug`, `info`, `warn`, `error` |
| `TURNSTILE_SECRET_KEY` | _(required)_       | Cloudflare Turnstile secret key             |
| `ALLOWED_ORIGINS`      | `*`                | Comma-separated CORS origins                |
| `RATE_LIMIT_RPM`       | `10`               | Rate limit requests per minute per IP       |
| `RATE_LIMIT_BURST`     | `15`               | Rate limit burst capacity                   |
| `READ_TIMEOUT`         | `10s`              | HTTP read timeout                           |
| `WRITE_TIMEOUT`        | `10s`              | HTTP write timeout                          |
| `IDLE_TIMEOUT`         | `60s`              | HTTP idle timeout                           |
| `SHUTDOWN_TIMEOUT`     | `30s`              | Graceful shutdown deadline                  |
| `VERSION`              | `dev`              | Version string reported in health checks    |
| `ADMIN_PASSWORD_HASH`  | _(not set)_        | Argon2id PHC hash for the admin password    |
| `SMTP_HOST`            | _(not set)_        | SMTP server hostname (empty = disabled)     |
| `SMTP_PORT`            | `587`              | SMTP server port                            |
| `SMTP_USERNAME`        | _(not set)_        | SMTP username for PLAIN auth                |
| `SMTP_PASSWORD`        | _(not set)_        | SMTP password for PLAIN auth                |
| `SMTP_FROM`            | _(not set)_        | Sender email address for notifications      |
| `SMTP_TO`              | _(not set)_        | Recipient email address for notifications   |

## API Endpoints

| Method | Path                    | Description                       |
| ------ | ----------------------- | --------------------------------- |
| `POST` | `/api/v1/comments`      | Create a comment                  |
| `GET`  | `/api/v1/comments`      | List approved comments            |
| `GET`  | `/api/v1/comments/{id}` | Get a comment by ID               |
| `GET`  | `/health`               | Health check with database status |
| `GET`  | `/debug/vars`           | Runtime metrics (expvar)          |

## Admin Interface

An HTML admin interface is available at `GET /admin` for managing comments. It uses a single-page design with AlpineJS loaded from CDN — no build pipeline required.

### Setup

Generate an Argon2id hash of your admin password:

```bash
just hash
```

Copy the output `ADMIN_PASSWORD_HASH=...` into your `.env` file. If `ADMIN_PASSWORD_HASH` is empty or unset, the interface still loads but mutating operations (toggle approval, delete) return `501 Not Implemented`.

### Usage

Open `http://localhost:8080/admin` in a browser. Four actions are available:

| Action | Endpoint | Auth Required |
| ------ | -------- | ------------- |
| Get a comment | `GET /api/v1/admin/comments/{id}` | No |
| List all comments | `GET /api/v1/admin/comments` | No |
| Toggle approval | `POST /api/v1/admin/comments/{id}/toggle-approval` | Password in JSON body |
| Delete a comment | `DELETE /api/v1/admin/comments/{id}` | Password in JSON body |

**Reading operations** (get, list) require no authentication and work for *all* comments regardless of approval state.

**Mutating operations** (toggle, delete) require the admin password sent in the request body: `{"password": "your-password"}`. The password is verified securely against the stored Argon2id hash with constant-time comparison.

The list view shows all comments with pagination (20 per page), an excerpt of the body (first 100 characters), the approval status, and an optional post ID filter. Results are rendered inline using AlpineJS — no page reloads.

### Email Notifications

When a new comment is created, an email notification can be sent to the administrator if SMTP is configured. The email includes the comment's numeric ID and a brief message indicating a new comment requires moderation.

| Variable | Description |
| -------- | ----------- |
| `SMTP_HOST` | SMTP server hostname. Leave empty to disable email notifications entirely. |
| `SMTP_PORT` | SMTP server port (default `587`). |
| `SMTP_USERNAME` | Username for PLAIN authentication against the SMTP server. |
| `SMTP_PASSWORD` | Password for SMTP authentication. |
| `SMTP_FROM` | The sender address used in the `From` header of notification emails. |
| `SMTP_TO` | The recipient address where notification emails are delivered. |

The email is sent **asynchronously** — the HTTP response is returned immediately, and failures are logged but never propagated to the client. If `SMTP_HOST` is empty, notification sending is silently skipped.

### Health Check Response

```json
{
  "status": "healthy",
  "version": "dev",
  "uptime_seconds": 123.456,
  "checks": {
    "database": {
      "status": "connected",
      "response_ms": 2
    }
  }
}
```

### Metrics

Runtime metrics are exposed at `GET /debug/vars` in JSON format via `expvar`:

| Metric                                 | Description                           |
| -------------------------------------- | ------------------------------------- |
| `comments_created_total`               | Total comment creation attempts       |
| `turnstile_verifications_total`        | Total Turnstile verification attempts |
| `turnstile_verifications_failed_total` | Failed Turnstile verifications        |
| `rate_limit_hits_total`                | Rate limit violations                 |
| `validation_errors_total`              | Input validation failures             |
| `panics_total`                         | Recovered panics                      |

## Structured Logging

All log output is JSON-formatted via `log/slog`. Each request gets a unique `request_id` propagated via the `X-Request-ID` response header and logged in all request-scoped entries.

## Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` for graceful shutdown:

- Drains in-flight requests (up to `SHUTDOWN_TIMEOUT`, default 30s)
- On a second signal, forces immediate shutdown
- Stops rate limiter cleanup goroutine
- Logs final metrics on shutdown

## Docker

### Build

```bash
docker build -t replyforge:latest .
```

### Run

```bash
docker run -d \
  --name replyforge \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e TURNSTILE_SECRET_KEY=your-secret-key \
  -e ALLOWED_ORIGINS=https://exampleblog.com \
  replyforge:latest
```

### Docker Compose

```yaml
services:
  replyforge:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    env_file:
      - .env
    restart: unless-stopped
```

## Standalone Binary

```bash
go build -ldflags="-s -w" -o bin/server ./cmd/server
./bin/server
```

Ensure the parent directory of `DATABASE_PATH` exists and is writable.

## Development

```bash
just fmt    # Format code with goimports
just lint   # Run golangci-lint
just test   # Run all tests with race detector
just build  # Build the server binary
just run    # Build and run
```
