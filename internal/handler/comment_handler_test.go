package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/confuzeus/replyforge/internal/config"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	"github.com/confuzeus/replyforge/internal/sanitizer"
	"github.com/confuzeus/replyforge/internal/service"
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

func newTestService(t *testing.T, db *sql.DB, turnstileOK bool) *service.CommentService {
	t.Helper()
	return service.NewCommentService(service.ServiceDependencies{
		Repository:   repository.NewCommentRepository(db),
		DisplayIDGen: model.NewDisplayIDGenerator(),
		Turnstile:    &stubTurnstileVerifier{ok: turnstileOK},
		Sanitizer:    sanitizer.NewSanitizer(),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
}

func newTestHandler(t *testing.T, db *sql.DB, turnstileOK bool) *CommentHandler {
	t.Helper()
	svc := newTestService(t, db, turnstileOK)
	return NewCommentHandler(HandlerDependencies{
		Service: svc,
		Logger:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
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

func TestCreate_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	body := `{"post_id":"post-1","author_name":"Alice","body":"Hello world","turnstile_token":"valid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	var data model.CommentResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, "post-1", data.PostID)
	assert.Equal(t, "Alice", data.AuthorName)
	assert.Equal(t, "Hello world", data.Body)
	assert.False(t, data.Approved)
	assert.NotEmpty(t, data.DisplayID)
}

func TestCreate_ValidationError(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	body := `{"post_id":"","author_name":"","body":"","turnstile_token":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp model.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "VALIDATION_ERROR", resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Details)
}

func TestCreate_TurnstileFailed(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, false)

	body := `{"post_id":"post-1","author_name":"Alice","body":"Hello world","turnstile_token":"bad"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp model.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "TURNSTILE_FAILED", resp.Error.Code)
}

func TestCreate_TurnstileError(t *testing.T) {
	db := setupTestDB(t)
	svc := service.NewCommentService(service.ServiceDependencies{
		Repository:   repository.NewCommentRepository(db),
		DisplayIDGen: model.NewDisplayIDGenerator(),
		Turnstile:    &stubTurnstileVerifier{err: errors.New("network error")},
		Sanitizer:    sanitizer.NewSanitizer(),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	handler := NewCommentHandler(HandlerDependencies{
		Service: svc,
		Logger:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})

	body := `{"post_id":"post-1","author_name":"Alice","body":"Hello world","turnstile_token":"valid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreate_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestList_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	seedApprovedComment(t, db, "post-1", "Alice", "First")
	seedApprovedComment(t, db, "post-1", "Bob", "Second")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 2, resp.Pagination.Total)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.PerPage)
}

func TestList_WithQueryParams(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	seedApprovedComment(t, db, "post-a", "Alice", "Comment")
	seedApprovedComment(t, db, "post-b", "Bob", "Comment")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments?post_id=post-a&page=1&per_page=10", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "post-a", resp.Data[0].PostID)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 10, resp.Pagination.PerPage)
}

func TestList_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Pagination.Total)
}

func TestGet_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	c := seedApprovedComment(t, db, "post-1", "Alice", "Hello world")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/"+formatID(c.ID), nil)
	req.SetPathValue("id", formatID(c.ID))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	var data model.CommentResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, c.ID, data.ID)
	assert.Equal(t, "Alice", data.AuthorName)
}

func TestGet_NotFound(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/99999", nil)
	req.SetPathValue("id", "99999")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp model.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
}

func TestGet_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp model.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "VALIDATION_ERROR", resp.Error.Code)
}

func TestCreate_SpecialCharactersPreserved(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	body := fmt.Sprintf(
		`{"post_id":"post-1","author_name":"O'Brien","body":%s,"turnstile_token":"valid"}`,
		toJSONStr("I'm happy & it's a < b test"),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	var data model.CommentResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, "O'Brien", data.AuthorName)
	assert.Equal(t, "I'm happy & it's a < b test", data.Body)
	assert.NotContains(t, rec.Body.String(), "\\u0026")
	assert.NotContains(t, rec.Body.String(), "&#39;")
}

func TestCreate_ResponseHasNoUnicodeEscapes(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	body := fmt.Sprintf(
		`{"post_id":"post-1","author_name":"Test","body":%s,"turnstile_token":"valid"}`,
		toJSONStr("<html> & \"quotes\""),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), "\\u003c")
	assert.NotContains(t, rec.Body.String(), "\\u003e")
	assert.NotContains(t, rec.Body.String(), "\\u0026")
}

func toJSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func TestList_WithSortAscending(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	seedApprovedComment(t, db, "post-1", "Alice", "First")
	time.Sleep(10 * time.Millisecond)
	seedApprovedComment(t, db, "post-1", "Bob", "Second")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments?sort=created_at", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Data, 2)
	assert.True(t, resp.Data[0].CreatedAt.Before(resp.Data[1].CreatedAt) ||
		resp.Data[0].CreatedAt.Equal(resp.Data[1].CreatedAt))
}

func TestList_QueryParamsEdgeCases(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	seedApprovedComment(t, db, "post-1", "Alice", "Comment")

	tests := []struct {
		name        string
		queryStr    string
		wantStatus  int
		wantPage    int
		wantPerPage int
	}{
		{"defaults", "", http.StatusOK, 1, 20},
		{"per_page over max clamped", "per_page=200", http.StatusOK, 1, 100},
		{"invalid page is ignored", "page=abc", http.StatusOK, 1, 20},
		{"negative page defaults", "page=-1", http.StatusOK, 1, 20},
		{"zero page defaults", "page=0", http.StatusOK, 1, 20},
		{"zero per_page defaults", "per_page=0", http.StatusOK, 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/comments?"+tt.queryStr, nil)
			rec := httptest.NewRecorder()

			handler.List(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var resp model.ListCommentsResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.Equal(t, tt.wantPage, resp.Pagination.Page)
			assert.Equal(t, tt.wantPerPage, resp.Pagination.PerPage)
		})
	}
}

func TestGet_NegativeID(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/-1", nil)
	req.SetPathValue("id", "-1")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGet_ZeroID(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestHandler(t, db, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/0", nil)
	req.SetPathValue("id", "0")
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
