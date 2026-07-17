# Testing Strategy

## Test Pyramid

```text
      ╱         ╲
    ╱   E2E      ╲        Manual testing / smoke tests
  ╱───────────────╲
╱   Integration     ╲     Handler + Service + real SQLite
╱─────────────────────╲
╱     Unit Tests        ╲  Models, validators, sanitizers, display ID
╱─────────────────────────╲
```

## Unit Tests

```go
// Model validation tests (table-driven)
func TestCreateCommentRequest_Validate(t *testing.T) {
    tests := []struct {
        name    string
        request CreateCommentRequest
        wantErr bool
        errFields []string
    }{
        {
            name: "valid request",
            request: CreateCommentRequest{
                PostID:     "my-post",
                AuthorName: "Jane Doe",
                Body:       "Great article!",
                TurnstileToken: "valid-token",
            },
            wantErr: false,
        },
        {
            name: "missing author name",
            request: CreateCommentRequest{
                PostID:     "my-post",
                AuthorName: "",
                Body:       "Great article!",
                TurnstileToken: "valid-token",
            },
            wantErr: true,
            errFields: []string{"author_name"},
        },
        // ... more test cases ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.request.Validate()
            // assertions...
        })
    }
}

// Display ID generation tests
func TestDisplayIDGenerator_Generate(t *testing.T) {
    gen := NewDisplayIDGenerator()

    id, err := gen.Generate(42)
    require.NoError(t, err)
    assert.Len(t, string(id), 6)
    assert.Regexp(t, "^[a-z0-9]+$", string(id))
}

// Sanitizer tests
func TestSanitizer_Sanitize(t *testing.T) {
    s := NewSanitizer()

    tests := []struct {
        input    string
        expected string
    }{
        {"<script>alert('xss')</script>", "alert(&#39;xss&#39;)"},
        {"  Hello   World  ", "Hello World"},
        {"Jöhn", "Jöhn"}, // Unicode preserved
    }

    for _, tt := range tests {
        result := s.Sanitize(tt.input)
        assert.Equal(t, tt.expected, result)
    }
}
```

## Integration Tests

```go
// Repository tests with in-memory SQLite
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)

    // Enable WAL mode
    _, err = db.Exec("PRAGMA journal_mode=WAL")
    require.NoError(t, err)

    // Run migrations
    err = migrations.Run(db)
    require.NoError(t, err)

    return db
}

func TestCommentRepository_Insert(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := NewCommentRepository(db)

    comment
```
