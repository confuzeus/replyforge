<div style="display: flex; align-items: center; gap: 1rem">
<img src="./assets/replyforge-logo.svg" alt="Reply Forge logo"/>
<h1>Reply Forge</h1>
</div>

A REST API service for managing blog comments with anti-spam protection via Cloudflare Turnstile. Comments are stored in SQLite and held for moderation before being served to readers.

## How It Works

Readers submit comments through a Turnstile-protected API endpoint. Comments are created in an unapproved state and stored in SQLite. Approved comments are served publicly via read-only endpoints. An admin interface at `/admin` allows moderators to review, approve, and delete comments. Email notifications can be sent to the admin when new comments arrive.

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

## Client API Reference

All endpoints return `Content-Type: application/json`. A unique `X-Request-ID` header is set on every response for tracing.

### Error Response Format

Errors follow a consistent structure:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "The request contains invalid parameters",
    "details": [{ "field": "author_name", "message": "is required" }]
  }
}
```

| Code               | HTTP Status | Description                                                                |
| ------------------ | ----------- | -------------------------------------------------------------------------- |
| `VALIDATION_ERROR` | `400`       | Request body failed validation. `details` array contains per-field errors. |
| `TURNSTILE_FAILED` | `403`       | Cloudflare Turnstile token was missing, invalid, or verification failed.   |
| `NOT_FOUND`        | `404`       | Comment does not exist or is not yet approved.                             |
| `INTERNAL_ERROR`   | `500`       | Unexpected server error. Retry later.                                      |

### Create a Comment

```text
POST /api/v1/comments
```

**Request body:**

```json
{
  "post_id": "my-blog-post-1",
  "author_name": "Alice",
  "body": "Great article, thanks for sharing!",
  "turnstile_token": "0x4AA...client-turnstile-token..."
}
```

| Field             | Type   | Max Length | Required | Description                                                      |
| ----------------- | ------ | ---------- | -------- | ---------------------------------------------------------------- |
| `post_id`         | string | 100        | yes      | Identifies which post the comment belongs to. Free-form string.  |
| `author_name`     | string | 100        | yes      | Display name of the commenter. Stripped of all HTML tags.        |
| `body`            | string | 5000       | yes      | Comment text. Stripped of all HTML tags.                         |
| `turnstile_token` | string | —          | yes      | Cloudflare Turnstile client-side token obtained from the widget. |

**Validation rules:**

- All four fields are required. Empty strings are rejected.
- `author_name` and `body` are sanitized server-side: all HTML tags stripped, trimmed, Unicode NFC normalized, whitespace collapsed, null bytes removed.
- `turnstile_token` must be a valid Cloudflare Turnstile token. Verification is cached in-memory for 5 minutes (keyed by SHA256(token + client IP)).

**Rate limiting:**

This endpoint is rate-limited per client IP (default: 10 requests per minute, burst of 15). Exceeding the limit returns `429 Too Many Requests`.

**Success response (`201 Created`):**

```json
{
  "data": {
    "id": 42,
    "display_id": "aB3xK7",
    "post_id": "my-blog-post-1",
    "author_name": "Alice",
    "body": "Great article, thanks for sharing!",
    "approved": false,
    "created_at": "2026-07-19T12:00:00Z"
  }
}
```

All comments are created with `approved: false` and require admin moderation before appearing in the public listing. The `display_id` is a short unique identifier (generated via sqids from the numeric `id`), suitable for use in URL fragments or client-side references.

### List Approved Comments

```text
GET /api/v1/comments?post_id=my-blog-post-1&page=1&per_page=20&sort=created_at
```

**Query parameters:**

| Parameter  | Type   | Default | Description                                                                                               |
| ---------- | ------ | ------- | --------------------------------------------------------------------------------------------------------- |
| `post_id`  | string | —       | Filter comments belonging to a specific post.                                                             |
| `page`     | int    | `1`     | Page number (1-indexed).                                                                                  |
| `per_page` | int    | `20`    | Items per page (max `100`).                                                                               |
| `sort`     | string | —       | Sort order. Set to `created_at` for ascending chronological order. Defaults to descending (newest first). |

**Success response (`200 OK`):**

```json
{
  "data": [
    {
      "id": 42,
      "display_id": "aB3xK7",
      "post_id": "my-blog-post-1",
      "author_name": "Alice",
      "body": "Great article, thanks for sharing!",
      "approved": true,
      "created_at": "2026-07-19T12:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

Only approved comments are returned. Unapproved comments are invisible to this endpoint.

### Get a Comment by ID

```text
GET /api/v1/comments/{id}
```

`{id}` is the numeric comment ID (e.g. `42`). Returns `404 Not Found` if the comment does not exist **or** is not yet approved.

**Success response (`200 OK`):**

```json
{
  "data": {
    "id": 42,
    "display_id": "aB3xK7",
    "post_id": "my-blog-post-1",
    "author_name": "Alice",
    "body": "Great article, thanks for sharing!",
    "approved": true,
    "created_at": "2026-07-19T12:00:00Z"
  }
}
```

## Admin Interface

An HTML admin interface for comment moderation is available at `GET /admin`. Generate an admin password hash with `just hash` and add `ADMIN_PASSWORD_HASH` to your `.env`. Four admin API endpoints are available:

| Action            | Endpoint                                           | Auth Required         |
| ----------------- | -------------------------------------------------- | --------------------- |
| Get a comment     | `GET /api/v1/admin/comments/{id}`                  | No                    |
| List all comments | `GET /api/v1/admin/comments`                       | No                    |
| Toggle approval   | `POST /api/v1/admin/comments/{id}/toggle-approval` | Password in JSON body |
| Delete a comment  | `DELETE /api/v1/admin/comments/{id}`               | Password in JSON body |

### Docker Compose

Available on [Docker Hub](https://hub.docker.com/repository/docker/dockershepherd/replyforge/general)

Example:

```yaml
services:
  replyforge:
    image: dockershepherd/replyforge:dev-36f3fc4
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
