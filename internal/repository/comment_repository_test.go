package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/confuzeus/replyforge/internal/config"
	"github.com/confuzeus/replyforge/migrations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	require.NoError(t, err)
	require.NoError(t, config.RunMigrations(db, migrations.SQL))
	return db
}

func seedComment(t *testing.T, db *sql.DB, c *Comment) {
	t.Helper()
	repo := NewCommentRepository(db)
	_, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)
}

func TestInsert_ReturnsID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	c := &Comment{
		PostID:     "post-1",
		AuthorName: "Alice",
		Body:       "Hello world",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}

	id, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
	assert.Equal(t, id, c.ID)
}

func TestInsert_SetDefaults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	c := &Comment{
		PostID:     "post-1",
		AuthorName: "Bob",
		Body:       "Minimal",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}

	id, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)

	retrieved, err := repo.FindByID(context.Background(), id)
	require.NoError(t, err)

	assert.False(t, retrieved.Approved)
	assert.Equal(t, "", retrieved.DisplayID)
	assert.False(t, retrieved.TurnstileVerified)
}

func TestUpdateDisplayID_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	c := &Comment{
		PostID:     "post-1",
		AuthorName: "Carol",
		Body:       "Update me",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}

	id, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)

	err = repo.UpdateDisplayID(context.Background(), id, "abc123")
	require.NoError(t, err)

	retrieved, err := repo.FindByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "abc123", retrieved.DisplayID)
}

func TestUpdateDisplayID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	err := repo.UpdateDisplayID(context.Background(), 9999, "abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFindByID_Found(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	now := time.Now().Truncate(time.Second)
	c := &Comment{
		PostID:            "post-1",
		AuthorName:        "Dave",
		Body:              "Full comment body",
		IPAddress:         "192.168.1.1",
		UserAgent:         "Firefox",
		TurnstileVerified: true,
	}

	id, err := repo.Insert(context.Background(), c)
	require.NoError(t, err)

	require.NoError(t, repo.UpdateDisplayID(context.Background(), id, "xyz789"))

	retrieved, err := repo.FindByID(context.Background(), id)
	require.NoError(t, err)

	assert.Equal(t, id, retrieved.ID)
	assert.Equal(t, "xyz789", retrieved.DisplayID)
	assert.Equal(t, "post-1", retrieved.PostID)
	assert.Equal(t, "Dave", retrieved.AuthorName)
	assert.Equal(t, "Full comment body", retrieved.Body)
	assert.False(t, retrieved.Approved)
	assert.Equal(t, "192.168.1.1", retrieved.IPAddress)
	assert.Equal(t, "Firefox", retrieved.UserAgent)
	assert.True(t, retrieved.TurnstileVerified)
	assert.WithinDuration(t, now, retrieved.CreatedAt, 2*time.Second)
	assert.WithinDuration(t, now, retrieved.UpdatedAt, 2*time.Second)
}

func TestFindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	_, err := repo.FindByID(context.Background(), 9999)
	assert.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestFindApproved_AllApproved(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	for i, approved := range []bool{true, true, true, false} {
		c := &Comment{
			PostID:            "post-1",
			AuthorName:        "User",
			Body:              "Comment",
			IPAddress:         "127.0.0.1",
			UserAgent:         "test",
			Approved:          approved,
			TurnstileVerified: true,
		}
		seedComment(t, db, c)
		_ = i
	}

	comments, total, err := repo.FindApproved(context.Background(), QueryParams{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, comments, 3)
}

func TestFindApproved_FilterByPost(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	for _, postID := range []string{"post-a", "post-a", "post-b"} {
		c := &Comment{
			PostID:            postID,
			AuthorName:        "User",
			Body:              "Comment",
			IPAddress:         "127.0.0.1",
			UserAgent:         "test",
			Approved:          true,
			TurnstileVerified: true,
		}
		seedComment(t, db, c)
	}

	comments, total, err := repo.FindApproved(context.Background(), QueryParams{
		PostID:  "post-a",
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, comments, 2)
	for _, c := range comments {
		assert.Equal(t, "post-a", c.PostID)
	}
}

func TestFindApproved_Pagination(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	for i := 0; i < 25; i++ {
		c := &Comment{
			PostID:            "post-1",
			AuthorName:        "User",
			Body:              "Comment",
			IPAddress:         "127.0.0.1",
			UserAgent:         "test",
			Approved:          true,
			TurnstileVerified: true,
		}
		seedComment(t, db, c)
	}

	comments, total, err := repo.FindApproved(context.Background(), QueryParams{
		Page:    2,
		PerPage: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, comments, 10)
}

func TestFindApproved_SortAsc(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	c1 := &Comment{
		PostID: "post-1", AuthorName: "User", Body: "First",
		IPAddress: "127.0.0.1", UserAgent: "test",
		Approved: true, TurnstileVerified: true,
	}
	seedComment(t, db, c1)

	c2 := &Comment{
		PostID: "post-1", AuthorName: "User", Body: "Second",
		IPAddress: "127.0.0.1", UserAgent: "test",
		Approved: true, TurnstileVerified: true,
	}
	seedComment(t, db, c2)

	comments, _, err := repo.FindApproved(context.Background(), QueryParams{
		Page:    1,
		PerPage: 10,
		Sort:    "created_at",
	})
	require.NoError(t, err)
	assert.Len(t, comments, 2)
	assert.True(t, comments[0].CreatedAt.Before(comments[1].CreatedAt) ||
		comments[0].CreatedAt.Equal(comments[1].CreatedAt))
}

func TestFindApproved_SortDesc(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	c1 := &Comment{
		PostID: "post-1", AuthorName: "User", Body: "First",
		IPAddress: "127.0.0.1", UserAgent: "test",
		Approved: true, TurnstileVerified: true,
	}
	seedComment(t, db, c1)

	c2 := &Comment{
		PostID: "post-1", AuthorName: "User", Body: "Second",
		IPAddress: "127.0.0.1", UserAgent: "test",
		Approved: true, TurnstileVerified: true,
	}
	seedComment(t, db, c2)

	comments, _, err := repo.FindApproved(context.Background(), QueryParams{
		Page:    1,
		PerPage: 10,
		Sort:    "-created_at",
	})
	require.NoError(t, err)
	assert.Len(t, comments, 2)
	assert.True(t, comments[0].CreatedAt.After(comments[1].CreatedAt) ||
		comments[0].CreatedAt.Equal(comments[1].CreatedAt))
}

func TestFindApproved_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	comments, total, err := repo.FindApproved(context.Background(), QueryParams{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, comments)
	assert.NotNil(t, comments)
}
