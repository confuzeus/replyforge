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

## API Endpoints

| Method | Path                    | Description                       |
| ------ | ----------------------- | --------------------------------- |
| `POST` | `/api/v1/comments`      | Create a comment                  |
| `GET`  | `/api/v1/comments`      | List approved comments            |
| `GET`  | `/api/v1/comments/{id}` | Get a comment by ID               |
| `GET`  | `/health`               | Health check with database status |
| `GET`  | `/debug/vars`           | Runtime metrics (expvar)          |

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
go build -ldflags="-s -w -X 'main.VERSION=1.0.0'" -o bin/server ./cmd/server
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
