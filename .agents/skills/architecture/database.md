# Database Architecture

## Database Configuration

PRAGMAs are split by application scope:

### Persistent (set once, survive across connections)
Applied via migration `000_prerequisites.sql`:
```sql
PRAGMA synchronous=NORMAL;
```

### Connection-level (applied on every connection open)
Applied via DSN query parameters in `main.go`:
```text
?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on
```

- `journal_mode=WAL` — write-ahead logging for concurrent reads + writes
- `busy_timeout=5000` — wait up to 5s before returning SQLITE_BUSY
- `foreign_keys=ON` — enforce foreign key constraints
- `synchronous=NORMAL` — balance durability vs performance (safe with WAL)

### Schema

#### Migration 001: Create Comments Table

```sql
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    display_id TEXT NOT NULL,
    post_id TEXT NOT NULL,
    author_name TEXT NOT NULL,
    body TEXT NOT NULL,
    approved INTEGER NOT NULL DEFAULT 0,
    ip_address TEXT NOT NULL,
    user_agent TEXT DEFAULT '',
    captcha_verified INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_comments_post_approved_created
ON comments(post_id, approved, created_at DESC);
```

**Schema Notes:**

- `id` is the internal auto-incrementing primary key, used for all database operations
- `display_id` is generated from the `id` value using Sqids
- `approved` defaults to 0 (false) - all comments require moderation or approval logic
- `ip_address` and `user_agent` are stored for abuse prevention, never returned in API responses
- `captcha_verified` tracks whether Cloudflare verification passed
- The composite index on `(post_id, approved, created_at DESC)` optimizes the most common query pattern

### Display ID Generation

```go
package model

import (
    "fmt"
    "github.com/sqids/sqids-go"
)

type DisplayIDGenerator struct {
    s *sqids.Sqids
}

func NewDisplayIDGenerator() *DisplayIDGenerator {
    s, err := sqids.New(sqids.Options{
        MinLength: 6,
    })
    if err != nil {
        panic(fmt.Sprintf("failed to create sqids instance: %v", err))
    }
    return &DisplayIDGenerator{s: s}
}

func (g *DisplayIDGenerator) Generate(id int64) (string, error) {
    encoded, err := g.s.Encode([]uint64{uint64(id)})
    if err != nil {
        return "", fmt.Errorf("encoding display id: %w", err)
    }
    return encoded, nil
}
```

**Generation Strategy:**

- Generated non-deterministically from the integer `id` using Sqids
- No salt needed — Sqids natively produces non-consecutive randomized IDs
- 6-character minimum length, alphanumeric
- Generated after database insert since it requires the auto-incremented ID
- Performed under the same mutex as the insert

### Data Flow

#### Comment Creation Flow

```
Client Request
    │
    ▼
[1] Input Validation & Sanitization
    │
    ▼
[2] Rate Limit Check (by IP)
    │
    ▼
[3] Turnstile Verification (Cloudflare API)
    │
    ▼
[4] Database Insert (approved=0, no display_id yet)
    │
    ▼
[5] Generate display_id from returned id
    │
    ▼
[6] Update comment with display_id (same transaction)
    │
    ▼
[7] Log structured event
    │
    ▼
[8] Response (201 Created)
```

#### Comment Retrieval Flow

```
Client Request
    │
    ▼
[1] Parse and validate path/query parameters
    │
    ▼
[2] Build SQL query with approved=1 filter
    │
    ▼
[3] Execute query with parameterized values
    │
    ▼
[4] Map results to response DTOs
    │
    ▼
[5] Calculate pagination metadata
    │
    ▼
[6] Response (200 OK)
```

### Concurrency Strategy

```go
type CommentService struct {
    repo    *CommentRepository
    mu      sync.Mutex  // Serializes all write operations
    logger  *slog.Logger
}

// Read operations: Concurrent (SQLite WAL mode supports this)
// Write operations: Acquire mutex before proceeding
func (s *CommentService) Create(ctx context.Context, input CreateInput) (*Comment, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // ... write operations ...
}
```

**Why a mutex instead of connection pooling?**

- SQLite serializes writes internally anyway
- Explicit mutex prevents SQLITE_BUSY errors at the application level
- Simpler than managing separate read/write connection pools
- Appropriate for blog-scale workloads (tens to hundreds of requests per second)

---
