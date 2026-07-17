# Component Architecture

## Project Structure

```text
blog-comments-api/
├── cmd/
│   └── server/
│       └── main.go                        # Application entry point
├── internal/
│   ├── config/
│   │   ├── config.go                      # Configuration loading & validation
│   │   └── config_test.go
│   ├── handler/
│   │   ├── comment_handler.go             # HTTP handlers
│   │   └── comment_handler_test.go
│   ├── middleware/
│   │   ├── cors.go                        # CORS middleware
│   │   ├── ratelimit.go                   # Rate limiting middleware
│   │   ├── logging.go                     # Request logging middleware
│   │   ├── recovery.go                    # Panic recovery middleware
│   │   └── middleware_test.go
│   ├── model/
│   │   ├── comment.go                     # Domain models & validation
│   │   ├── display_id.go                  # Display ID generation
│   │   └── model_test.go
│   ├── repository/
│   │   ├── comment_repository.go          # SQLite database operations
│   │   └── comment_repository_test.go
│   ├── service/
│   │   ├── comment_service.go             # Business logic
│   │   ├── turnstile.go                   # Cloudflare Turnstile verification
│   │   └── service_test.go
│   └── sanitizer/
│       ├── sanitizer.go                   # Input sanitization
│       └── sanitizer_test.go
├── migrations/
│   └── 001_create_comments.sql            # Database schema
├── go.mod
├── go.sum
└── README.md
```

## Component Interfaces

### Handler Layer

```go
type CommentHandler struct {
    service *service.CommentService
    logger  *slog.Logger
}

type HandlerDependencies struct {
    Service *service.CommentService
    Logger  *slog.Logger
}

func NewCommentHandler(deps HandlerDependencies) *CommentHandler {
    return &CommentHandler{
        service: deps.Service,
        logger:  deps.Logger,
    }
}

func (h *CommentHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("POST /api/v1/comments", h.Create)
    mux.HandleFunc("GET /api/v1/comments", h.List)
    mux.HandleFunc("GET /api/v1/comments/{id}", h.Get)
}
```

### Service Layer

```go
type CommentService struct {
    repo             *repository.CommentRepository
    displayIDGen     *model.DisplayIDGenerator
    turnstileClient  *TurnstileVerifier
    sanitizer        *sanitizer.Sanitizer
    mu               sync.Mutex
    logger           *slog.Logger
}

type CreateInput struct {
    PostID         string
    AuthorName     string
    Body           string
    TurnstileToken string
    ClientIP       string
    UserAgent      string
}

type ListParams struct {
    PostID  string
    Page    int
    PerPage int
    Sort    string
}

type ListResult struct {
    Comments   []*model.CommentResponse
    Total      int
    Page       int
    PerPage    int
    TotalPages int
}

func NewCommentService(deps ServiceDependencies) *CommentService {
    return &CommentService{
        repo:            deps.Repository,
        displayIDGen:    deps.DisplayIDGenerator,
        turnstileClient: deps.TurnstileVerifier,
        sanitizer:       deps.Sanitizer,
        logger:          deps.Logger,
    }
}

func (s *CommentService) Create(ctx context.Context, input CreateInput) (*model.CommentResponse, error)
func (s *CommentService) List(ctx context.Context, params ListParams) (*ListResult, error)
func (s *CommentService) GetByID(ctx context.Context, id int64) (*model.CommentResponse, error)
```

### Repository Layer

```go
type CommentRepository struct {
    db *sql.DB
}

type Comment struct {
    ID                int64
    DisplayID         string
    PostID            string
    AuthorName        string
    Body              string
    Approved          bool
    IPAddress         string
    UserAgent         string
    TurnstileVerified bool
    CreatedAt         time.Time
    UpdatedAt         time.Time
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
    return &CommentRepository{db: db}
}

func (r *CommentRepository) Insert(ctx context.Context, comment *Comment) (int64, error)
func (r *CommentRepository) UpdateDisplayID(ctx context.Context, id int64, displayID string) error
func (r *CommentRepository) FindApproved(ctx context.Context, params QueryParams) ([]*Comment, int, error)
func (r *CommentRepository) FindByID(ctx context.Context, id int64) (*Comment, error)
```

### Domain Models

```go
type CommentResponse struct {
    ID          int64     `json:"id"`
    DisplayID   string    `json:"display_id"`
    PostID      string    `json:"post_id"`
    AuthorName  string    `json:"author_name"`
    Body        string    `json:"body"`
    Approved    bool      `json:"approved"`
    CreatedAt   time.Time `json:"created_at"`
}

type CreateCommentRequest struct {
    PostID         string `json:"post_id"`
    AuthorName     string `json:"author_name"`
    Body           string `json:"body"`
    TurnstileToken string `json:"turnstile_token"`
}

type ListCommentsResponse struct {
    Data       []*CommentResponse `json:"data"`
    Pagination PaginationMeta     `json:"pagination"`
}

type PaginationMeta struct {
    Page       int `json:"page"`
    PerPage    int `json:"per_page"`
    Total      int `json:"total"`
    TotalPages int `json:"total_pages"`
}

type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string        `json:"code"`
    Message string        `json:"message"`
    Details []FieldError  `json:"details,omitempty"`
}
```

### Middleware Chain

```go
// Order matters - first middleware is outermost
func setupMiddleware(handler http.Handler, deps MiddlewareDependencies) http.Handler {
    handler = middleware.Recovery(deps.Logger)(handler)
    handler = middleware.Logging(deps.Logger)(handler)
    handler = middleware.CORS(deps.CORSConfig)(handler)
    handler = middleware.RateLimit(deps.RateLimiter)(handler)
    return handler
}
```

### Dependency Injection

```go
func main() {
    // Load configuration
    cfg := config.Load()

    // Initialize logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: cfg.LogLevel,
    }))

    // Initialize database
    db, err := sql.Open("sqlite3", cfg.DatabasePath)
    if err != nil {
        logger.Error("failed to open database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    // Run migrations
    if err := migrations.Run(db); err != nil {
        logger.Error("failed to run migrations", "error", err)
        os.Exit(1)
    }

    // Initialize repository
    commentRepo := repository.NewCommentRepository(db)

    // Initialize display ID generator
    displayIDGen := model.NewDisplayIDGenerator()

    // Initialize turnstile verifier
    turnstileClient := service.NewTurnstileVerifier(cfg.TurnstileSecretKey)

    // Initialize sanitizer
    sanitizer := sanitizer.NewSanitizer()

    // Initialize service
    commentService := service.NewCommentService(service.ServiceDependencies{
        Repository:       commentRepo,
        DisplayIDGenerator: displayIDGen,
        TurnstileVerifier:  turnstileClient,
        Sanitizer:         sanitizer,
        Logger:            logger,
    })

    // Initialize handler
    commentHandler := handler.NewCommentHandler(handler.HandlerDependencies{
        Service: commentService,
        Logger:  logger,
    })

    // Setup router
    mux := http.NewServeMux()
    commentHandler.RegisterRoutes(mux)

    // Add health check
    mux.HandleFunc("GET /health", healthCheck(db))

    // Apply middleware
    handler := setupMiddleware(mux, middlewareDeps)

    // Start server
    server := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      handler,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Graceful shutdown
    gracefulShutdown(server, db, logger)
}
```
