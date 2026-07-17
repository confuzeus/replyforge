package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/confuzeus/replyforge/internal/config"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	"github.com/confuzeus/replyforge/internal/sanitizer"
	"github.com/confuzeus/replyforge/migrations"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTurnstileVerifier struct {
	ok  bool
	err error
}

func (s *stubTurnstileVerifier) Verify(_ context.Context, _, _ string) (bool, error) {
	return s.ok, s.err
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	require.NoError(t, err)
	require.NoError(t, config.RunMigrations(db, migrations.SQL))
	return db
}

func newTestService(t *testing.T, db *sql.DB, turnstileOK bool) *CommentService {
	t.Helper()
	return NewCommentService(ServiceDependencies{
		Repository:   repository.NewCommentRepository(db),
		DisplayIDGen: model.NewDisplayIDGenerator(),
		Turnstile:    &stubTurnstileVerifier{ok: turnstileOK},
		Sanitizer:    sanitizer.NewSanitizer(),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
}

func seedApprovedComment(t *testing.T, db *sql.DB, postID, authorName, body string) *repository.Comment {
	t.Helper()
	repo := repository.NewCommentRepository(db)
	c := &repository.Comment{
		PostID:            postID,
		AuthorName:        authorName,
		Body:              body,
		Approved:          true,
		TurnstileVerified: true,
		IPAddress:         "127.0.0.1",
		UserAgent:         "test-agent",
	}
	id, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)

	gen := model.NewDisplayIDGenerator()
	displayID, err := gen.Generate(id)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateDisplayID(context.Background(), id, displayID))

	c.ID = id
	c.DisplayID = displayID
	return c
}

func seedUnapprovedComment(t *testing.T, db *sql.DB) *repository.Comment {
	t.Helper()
	repo := repository.NewCommentRepository(db)
	c := &repository.Comment{
		PostID:     "post-1",
		AuthorName: "Unapproved",
		Body:       "Unapproved comment",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}
	_, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)
	return c
}

func TestCreate_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	resp, err := svc.Create(context.Background(), CreateInput{
		PostID:         "post-1",
		AuthorName:     "Alice",
		Body:           "Hello world",
		TurnstileToken: "valid-token",
		ClientIP:       "127.0.0.1",
		UserAgent:      "test-agent",
	})

	require.NoError(t, err)
	assert.NotZero(t, resp.ID)
	assert.NotEmpty(t, resp.DisplayID)
	assert.Equal(t, "post-1", resp.PostID)
	assert.Equal(t, "Alice", resp.AuthorName)
	assert.Equal(t, "Hello world", resp.Body)
	assert.False(t, resp.Approved)
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCreate_TurnstileFailed(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, false)

	_, err := svc.Create(context.Background(), CreateInput{
		PostID:         "post-1",
		AuthorName:     "Alice",
		Body:           "Hello world",
		TurnstileToken: "bad-token",
		ClientIP:       "127.0.0.1",
		UserAgent:      "test-agent",
	})

	require.Error(t, err)
	var svcErr *ServiceError
	assert.True(t, errors.As(err, &svcErr))
	assert.Equal(t, "TURNSTILE_FAILED", svcErr.Code)
}

func TestCreate_TurnstileError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(ServiceDependencies{
		Repository:   repository.NewCommentRepository(db),
		DisplayIDGen: model.NewDisplayIDGenerator(),
		Turnstile:    &stubTurnstileVerifier{err: errors.New("network error")},
		Sanitizer:    sanitizer.NewSanitizer(),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})

	_, err := svc.Create(context.Background(), CreateInput{
		PostID:         "post-1",
		AuthorName:     "Alice",
		Body:           "Hello world",
		TurnstileToken: "valid-token",
		ClientIP:       "127.0.0.1",
		UserAgent:      "test-agent",
	})

	require.Error(t, err)
	var svcErr *ServiceError
	assert.True(t, errors.As(err, &svcErr))
	assert.Equal(t, "TURNSTILE_FAILED", svcErr.Code)
}

func TestCreate_SanitizedFields(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	resp, err := svc.Create(context.Background(), CreateInput{
		PostID:         "post-1",
		AuthorName:     "Alice<script>alert('xss')</script>",
		Body:           "<b>Hello</b> <script>alert('xss')</script>",
		TurnstileToken: "valid-token",
		ClientIP:       "127.0.0.1",
		UserAgent:      "test-agent",
	})

	require.NoError(t, err)
	assert.NotContains(t, resp.AuthorName, "<script>")
	assert.NotContains(t, resp.Body, "<script>")
	assert.NotContains(t, resp.Body, "<b>")
	assert.NotEmpty(t, resp.Body)
}

func TestList_Defaults(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	for i := 0; i < 5; i++ {
		seedApprovedComment(t, db, "post-1", "Alice", "Comment")
	}

	result, err := svc.List(context.Background(), ListParams{})

	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PerPage)
	assert.Equal(t, 1, result.TotalPages)
	assert.Len(t, result.Comments, 5)
}

func TestList_FilterByPost(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	seedApprovedComment(t, db, "post-a", "Alice", "Comment")
	seedApprovedComment(t, db, "post-a", "Bob", "Comment")
	seedApprovedComment(t, db, "post-b", "Carol", "Comment")

	result, err := svc.List(context.Background(), ListParams{PostID: "post-a"})

	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Comments, 2)
	for _, c := range result.Comments {
		assert.Equal(t, "post-a", c.PostID)
	}
}

func TestList_Pagination(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	for i := 0; i < 25; i++ {
		seedApprovedComment(t, db, "post-1", "Alice", "Comment")
	}

	result, err := svc.List(context.Background(), ListParams{Page: 2, PerPage: 10})

	require.NoError(t, err)
	assert.Equal(t, 25, result.Total)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 10, result.PerPage)
	assert.Equal(t, 3, result.TotalPages)
	assert.Len(t, result.Comments, 10)
}

func TestList_SortAscending(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	c1 := seedApprovedComment(t, db, "post-1", "Alice", "First")
	time.Sleep(10 * time.Millisecond)
	c2 := seedApprovedComment(t, db, "post-1", "Bob", "Second")

	result, err := svc.List(context.Background(), ListParams{Sort: "created_at"})

	require.NoError(t, err)
	assert.Len(t, result.Comments, 2)
	assert.True(t, result.Comments[0].ID == c1.ID)
	assert.True(t, result.Comments[1].ID == c2.ID)
}

func TestList_SortDescending(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	seedApprovedComment(t, db, "post-1", "Alice", "First")
	time.Sleep(10 * time.Millisecond)
	seedApprovedComment(t, db, "post-1", "Bob", "Second")

	result, err := svc.List(context.Background(), ListParams{Sort: "-created_at"})

	require.NoError(t, err)
	assert.Len(t, result.Comments, 2)
	assert.True(t, result.Comments[0].CreatedAt.After(result.Comments[1].CreatedAt) ||
		result.Comments[0].CreatedAt.Equal(result.Comments[1].CreatedAt))
}

func TestList_PerPageClamped(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	result, err := svc.List(context.Background(), ListParams{PerPage: 200})

	require.NoError(t, err)
	assert.Equal(t, 100, result.PerPage)
}

func TestList_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	result, err := svc.List(context.Background(), ListParams{})

	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Comments)
	assert.NotNil(t, result.Comments)
}

func TestGetByID_Found(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	c := seedApprovedComment(t, db, "post-1", "Alice", "Hello world")

	resp, err := svc.GetByID(context.Background(), c.ID)

	require.NoError(t, err)
	assert.Equal(t, c.ID, resp.ID)
	assert.Equal(t, c.DisplayID, resp.DisplayID)
	assert.Equal(t, "Alice", resp.AuthorName)
	assert.Equal(t, "Hello world", resp.Body)
	assert.True(t, resp.Approved)
}

func TestGetByID_NotApproved(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	c := seedUnapprovedComment(t, db)

	_, err := svc.GetByID(context.Background(), c.ID)

	require.Error(t, err)
	var svcErr *ServiceError
	assert.True(t, errors.As(err, &svcErr))
	assert.Equal(t, "NOT_FOUND", svcErr.Code)
}

func TestGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := newTestService(t, db, true)

	_, err := svc.GetByID(context.Background(), 99999)

	require.Error(t, err)
	var svcErr *ServiceError
	assert.True(t, errors.As(err, &svcErr))
	assert.Equal(t, "NOT_FOUND", svcErr.Code)
}
