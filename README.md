<img src="./assets/replyforge-logo.svg" width="60" alt="Reply Forge logo"/>

# Reply Forge

A REST API service for managing blog comments with anti-spam protection via Cloudflare Turnstile or p-captcha (a self-hosted proof-of-work alternative). Comments are stored in SQLite and held for moderation before being served to readers.

## How It Works

Readers submit comments through a captcha-protected API endpoint. Comments are created in an unapproved state and stored in SQLite. Approved comments are served publicly via read-only endpoints. An admin interface at `/admin` allows moderators to review, approve, and delete comments. Email notifications can be sent to the admin when new comments arrive.

Two captcha providers are supported:

- **Cloudflare Turnstile** (default) — invisible challenge, requires a Turnstile site key and secret key.
- **p-captcha** — a zero-dependency, self-hosted proof-of-work captcha. The client solves a Quadratic Residue Problem before submitting. No external API calls or third-party keys needed.

## Quick Start

1. **Generate an admin password hash:**

   ```bash
   docker run -it dockershepherd/replyforge:latest hash-password
   ```

   Copy the output `ADMIN_PASSWORD_HASH=...` line into your environment file.

2. **Create `.env.production`:**

   ```env
   # Cloudflare Turnstile (default)
   CAPTCHA_PROVIDER=turnstile
   TURNSTILE_SECRET_KEY=your-turnstile-secret-key

   # Or use p-captcha instead (no Turnstile key needed):
   # CAPTCHA_PROVIDER=pcaptcha
   # CAPTCHA_WOODALL=md
   # CAPTCHA_ROUNDS=2

   ADMIN_PASSWORD_HASH='$argon2id$v=19$m=65536,t=3,p=4$...'
   ALLOWED_ORIGINS=https://comments.example.com
   ```

3. **Create `conf/Caddyfile`:**

   ```caddy
   comments.example.com {
       reverse_proxy comments:8080
   }
   ```

4. **Create `docker-compose.yml`:**

   ```yaml
   name: replyforge

   volumes:
     comments_data:
     caddy_data:
     caddy_config:

   services:
     comments:
       image: dockershepherd/replyforge:latest
       restart: unless-stopped
       env_file:
         - path: ./.env.production
           required: true
       volumes:
         - comments_data:/app/data

     web:
       image: caddy:2-alpine
       restart: unless-stopped
       ports:
         - 80:80
         - 443:443
         - 443:443/udp
       volumes:
         - ./conf:/etc/caddy
         - caddy_data:/data
         - caddy_config:/config
   ```

5. **Start the services:**

   ```bash
   docker compose up -d
   ```

Read the [full deployment guide](https://replyforge.joshkaramuth.com/deploy-uncloud/).

## Configuration

Configuration is loaded from environment variables. Copy `.env.example` to `.env` and adjust as needed.

| Variable               | Default                    | Description                                                                     |
| ---------------------- | -------------------------- | ------------------------------------------------------------------------------- |
| `PORT`                 | `8080`                     | Server listen port                                                              |
| `DATABASE_PATH`        | `data/comments.db`         | SQLite database file path                                                       |
| `LOG_LEVEL`            | `info`                     | Log level: `debug`, `info`, `warn`, `error`                                     |
| `TURNSTILE_SECRET_KEY` | _(required for Turnstile)_ | Cloudflare Turnstile secret key (only needed when `CAPTCHA_PROVIDER=turnstile`) |
| `CAPTCHA_PROVIDER`     | `turnstile`                | Captcha provider: `turnstile` or `pcaptcha`                                     |
| `CAPTCHA_WOODALL`      | `md`                       | p-captcha proof-of-work difficulty alias (2xs, xs, sm, md, lg, xl, 2xl, 3xl)    |
| `CAPTCHA_ROUNDS`       | `2`                        | p-captcha number of proof-of-work problems to solve (max 20)                    |
| `ALLOWED_ORIGINS`      | `*`                        | Comma-separated CORS origins                                                    |
| `RATE_LIMIT_RPM`       | `10`                       | Rate limit requests per minute per IP                                           |
| `RATE_LIMIT_BURST`     | `15`                       | Rate limit burst capacity                                                       |
| `READ_TIMEOUT`         | `10s`                      | HTTP read timeout                                                               |
| `WRITE_TIMEOUT`        | `10s`                      | HTTP write timeout                                                              |
| `IDLE_TIMEOUT`         | `60s`                      | HTTP idle timeout                                                               |
| `SHUTDOWN_TIMEOUT`     | `30s`                      | Graceful shutdown deadline                                                      |
| `VERSION`              | `dev`                      | Version string reported in health checks                                        |
| `ADMIN_PASSWORD_HASH`  | _(not set)_                | Argon2id PHC hash for the admin password                                        |
| `SMTP_HOST`            | _(not set)_                | SMTP server hostname (empty = disabled)                                         |
| `SMTP_PORT`            | `587`                      | SMTP server port                                                                |
| `SMTP_USERNAME`        | _(not set)_                | SMTP username for PLAIN auth                                                    |
| `SMTP_PASSWORD`        | _(not set)_                | SMTP password for PLAIN auth                                                    |
| `SMTP_FROM`            | _(not set)_                | Sender email address for notifications                                          |
| `SMTP_TO`              | _(not set)_                | Recipient email address for notifications                                       |

## API Endpoints

| Method | Path                        | Description                                                                          |
| ------ | --------------------------- | ------------------------------------------------------------------------------------ |
| `POST` | `/api/v1/comments`          | Create a comment                                                                     |
| `GET`  | `/api/v1/comments`          | List approved comments                                                               |
| `GET`  | `/api/v1/comments/{id}`     | Get a comment by ID                                                                  |
| `GET`  | `/api/v1/captcha/challenge` | Generate a p-captcha proof-of-work challenge (only when `CAPTCHA_PROVIDER=pcaptcha`) |
| `GET`  | `/health`                   | Health check with database status                                                    |
| `GET`  | `/debug/vars`               | Runtime metrics (expvar)                                                             |

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

| Code               | HTTP Status | Description                                                                        |
| ------------------ | ----------- | ---------------------------------------------------------------------------------- |
| `VALIDATION_ERROR` | `400`       | Request body failed validation. `details` array contains per-field errors.         |
| `CAPTCHA_FAILED`   | `403`       | Captcha verification failed. The token or answer was invalid, missing, or expired. |
| `NOT_FOUND`        | `404`       | Comment does not exist or is not yet approved.                                     |
| `INTERNAL_ERROR`   | `500`       | Unexpected server error. Retry later.                                              |

### Create a Comment

```text
POST /api/v1/comments
```

**Request body (Turnstile):**

```json
{
  "post_id": "my-blog-post-1",
  "author_name": "Alice",
  "body": "Great article, thanks for sharing!",
  "turnstile_token": "0x4AA...client-turnstile-token..."
}
```

**Request body (p-captcha):**

```json
{
  "post_id": "my-blog-post-1",
  "author_name": "Alice",
  "body": "Great article, thanks for sharing!",
  "captcha_id": "abc123def456...",
  "captcha_answer": "base64-encoded-answers..."
}
```

| Field             | Type   | Max Length | Required       | Description                                                      |
| ----------------- | ------ | ---------- | -------------- | ---------------------------------------------------------------- |
| `post_id`         | string | 100        | yes            | Identifies which post the comment belongs to. Free-form string.  |
| `author_name`     | string | 100        | yes            | Display name of the commenter. Stripped of all HTML tags.        |
| `body`            | string | 5000       | yes            | Comment text. Stripped of all HTML tags.                         |
| `turnstile_token` | string | —          | Turnstile only | Cloudflare Turnstile client-side token obtained from the widget. |
| `captcha_id`      | string | —          | p-captcha only | Challenge ID returned from `GET /api/v1/captcha/challenge`.      |
| `captcha_answer`  | string | —          | p-captcha only | Base64-encoded solution to the proof-of-work challenge.          |

**Validation rules:**

- `post_id`, `author_name`, and `body` are required. Empty strings are rejected.
- When `CAPTCHA_PROVIDER=turnstile`: `turnstile_token` is required, must be a valid Cloudflare Turnstile token. Verification is cached in-memory for 5 minutes (keyed by SHA256(token + client IP)).
- When `CAPTCHA_PROVIDER=pcaptcha`: `captcha_id` and `captcha_answer` are required. The answer must be the correct solution to a previously generated challenge. Each challenge ID is single-use and expires after 10 minutes.
- `author_name` and `body` are sanitized server-side: all HTML tags stripped, trimmed, Unicode NFC normalized, whitespace collapsed, null bytes removed.

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

### p-captcha Challenge Endpoint

When using `CAPTCHA_PROVIDER=pcaptcha`, the client must first request a proof-of-work challenge, solve it, and submit the solution with the comment.

```text
GET /api/v1/captcha/challenge?woodall=md&rounds=2
```

**Query parameters:**

| Parameter | Type   | Default | Description                                                       |
| --------- | ------ | ------- | ----------------------------------------------------------------- |
| `woodall` | string | config  | Woodall prime alias (2xs–3xl). Controls proof-of-work difficulty. |
| `rounds`  | int    | config  | Number of problems to solve (1–20).                               |

**Success response (`200 OK`):**

```json
{
  "id": "d41d8cd98f00b204e9800998ecf8427e",
  "challenge": "QuadraticResidueProblem,base64-encoded-challenge-data..."
}
```

**Client-side flow:**

1. Request a challenge from `GET /api/v1/captcha/challenge`.
2. Solve the challenge using the [@p-captcha/react](https://www.npmjs.com/package/@p-captcha/react) widget or a compatible solver (Tonelli-Shanks algorithm for modular square roots).
3. Submit the solution as `captcha_id` (the `id` from the response) and `captcha_answer` (the base64-encoded solution) in the `POST /api/v1/comments` request body.

**Proof-of-work difficulty (on Apple M2 Pro):**

| Woodall Prime  | Alias | Bits   | Solve Time |
| -------------- | ----- | ------ | ---------- |
| 751×2⁷⁵¹−1     | 2xs   | 761    | ~80 ms     |
| 83×2⁵³¹⁸−1     | xs    | 5,322  | 238 ms     |
| 7755×2⁷⁷⁵⁵−1   | sm    | 7,765  | 598 ms     |
| 9531×2⁹⁵³¹−1   | md    | 9,542  | 931 ms     |
| 12379×2¹²³⁷⁹−1 | lg    | 12,387 | 1,995 ms   |
| 7911×2¹⁵⁸²³−1  | xl    | 15,830 | 3,466 ms   |
| 18885×2¹⁸⁸⁸⁵−1 | 2xl   | 18,891 | 5,581 ms   |
| 22971×2²²⁹⁷¹−1 | 3xl   | 22,974 | 9,199 ms   |

Higher difficulty takes longer for clients to solve but makes automated abuse more expensive. The default (`md`) takes about 930 ms per round — a 2-round challenge requires ~2 seconds of client CPU time.

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

An HTML admin interface for comment moderation is available at `GET /admin`. Generate an admin password hash with `docker run -it dockershepherd/replyforge:latest hash-password` and add `ADMIN_PASSWORD_HASH` to your `.env`. Four admin API endpoints are available:

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

## Credits

p-captcha was ported from [https://github.com/renton4code/p-captcha](https://github.com/renton4code/p-captcha)

## License

[AGPL 3](https://github.com/confuzeus/replyforge/blob/master/LICENSE)
