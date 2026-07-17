# Observability

## Health Check Endpoint

```text
GET /health

Response 200:
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "checks": {
    "database": {
      "status": "connected",
      "response_ms": 2
    }
  }
}
```

## Key Metrics (Exposed via /metrics or logging)

| Metric Name                            | Type      | Description                 |
| -------------------------------------- | --------- | --------------------------- |
| `comments_created_total`               | Counter   | Total creation attempts     |
| `comments_created_success_total`       | Counter   | Successful creations        |
| `turnstile_verifications_total`        | Counter   | Verification attempts       |
| `turnstile_verifications_failed_total` | Counter   | Failed verifications        |
| `request_duration_seconds`             | Histogram | Request latency by endpoint |
| `rate_limit_hits_total`                | Counter   | Rate limit violations by IP |
| `validation_errors_total`              | Counter   | Input validation failures   |
| `database_query_duration_seconds`      | Histogram | SQLite query performance    |

## Structured Logging

```go
// Use JSON-formatted structured logging throughout
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Include relevant context in all log entries
logger.Info("request processed",
    "method", r.Method,
    "path", r.URL.Path,
    "status", statusCode,
    "duration_ms", duration.Milliseconds(),
    "client_ip", hashIP(clientIP),
    "user_agent", r.UserAgent(),
)
```

## Monitoring Alerts

- **Error rate > 5%** - Investigation required
- **Turnstile failure rate > 10%** - Possible attack or configuration issue
- **Database query duration > 100ms** - Performance degradation
- **Rate limit hits spike** - Possible abuse attempt
