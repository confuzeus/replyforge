# Error Handling

## Error Taxonomy

| Code                  | HTTP Status | Description                   | When                                                 |
| --------------------- | ----------- | ----------------------------- | ---------------------------------------------------- |
| `VALIDATION_ERROR`    | 400         | Invalid input parameters      | Missing fields, invalid formats, out-of-range values |
| `NOT_FOUND`           | 404         | Resource not found            | Comment doesn't exist or isn't approved              |
| `TURNSTILE_FAILED`    | 403         | Anti-spam verification failed | Cloudflare rejected the token                        |
| `RATE_LIMIT_EXCEEDED` | 429         | Too many requests             | IP exceeded request limit                            |
| `INTERNAL_ERROR`      | 500         | Unexpected server error       | Database failures, unhandled panics                  |

## Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "The request contains invalid parameters",
    "details": [
      {
        "field": "author_name",
        "message": "author_name is required and must not be empty"
      },
      {
        "field": "body",
        "message": "body must not exceed 5000 characters"
      }
    ]
  }
}
```

## Error Handling Implementation

```go
func writeError(w http.ResponseWriter, statusCode int, errorCode string, message string, details []FieldError) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    resp := ErrorResponse{
        Error: ErrorDetail{
            Code:    errorCode,
            Message: message,
            Details: details,
        },
    }

    json.NewEncoder(w).Encode(resp)
}

// Usage in handlers:
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateCommentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
            "Invalid JSON body", nil)
        return
    }

    // ... validation and processing ...
}
```

## Logging Errors

```go
// Structured error logging (never expose these details to clients)
slog.Error("comment creation failed",
    "error", err.Error(),
    "post_id", input.PostID,
    "author_name", input.AuthorName,
    "ip", hashIP(input.ClientIP),
    "trace_id", traceID,
    "duration_ms", duration.Milliseconds(),
)

// Structured success logging
slog.Info("comment created",
    "comment_id", comment.ID,
    "display_id", comment.DisplayID,
    "post_id", comment.PostID,
    "duration_ms", duration.Milliseconds(),
    "captcha_verified", true,
)
```
