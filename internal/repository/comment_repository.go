package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

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

type QueryParams struct {
	PostID  string
	Page    int
	PerPage int
	Sort    string
}

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func scanComment(row interface{ Scan(...interface{}) error }) (*Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.DisplayID, &c.PostID, &c.AuthorName,
		&c.Body, &c.Approved, &c.IPAddress, &c.UserAgent,
		&c.TurnstileVerified, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CommentRepository) Insert(ctx context.Context, comment *Comment) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO comments (post_id, author_name, body, approved, ip_address, user_agent, turnstile_verified)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		comment.PostID, comment.AuthorName, comment.Body, comment.Approved,
		comment.IPAddress, comment.UserAgent, comment.TurnstileVerified,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting comment: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}
	comment.ID = id
	return id, nil
}

func (r *CommentRepository) UpdateDisplayID(ctx context.Context, id int64, displayID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE comments SET display_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		displayID, id,
	)
	if err != nil {
		return fmt.Errorf("updating display_id for comment %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}

func (r *CommentRepository) FindByID(ctx context.Context, id int64) (*Comment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, display_id, post_id, author_name, body, approved,
		 ip_address, user_agent, turnstile_verified, created_at, updated_at
		 FROM comments WHERE id = ?`, id,
	)
	return scanComment(row)
}

func (r *CommentRepository) FindByIDApproved(ctx context.Context, id int64) (*Comment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, display_id, post_id, author_name, body, approved,
		 ip_address, user_agent, turnstile_verified, created_at, updated_at
		 FROM comments WHERE id = ? AND approved = 1`, id,
	)
	return scanComment(row)
}

func (r *CommentRepository) FindApproved(ctx context.Context, params QueryParams) ([]*Comment, int, error) {
	where := "WHERE approved = 1"
	args := []interface{}{}
	if params.PostID != "" {
		where += " AND post_id = ?"
		args = append(args, params.PostID)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM comments " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting approved comments: %w", err)
	}

	orderBy := "ORDER BY created_at DESC"
	if params.Sort == "created_at" {
		orderBy = "ORDER BY created_at ASC"
	}

	offset := (params.Page - 1) * params.PerPage
	limit := params.PerPage

	selectQuery := fmt.Sprintf(
		`SELECT id, display_id, post_id, author_name, body, approved,
		 ip_address, user_agent, turnstile_verified, created_at, updated_at
		 FROM comments %s %s LIMIT ? OFFSET ?`,
		where, orderBy,
	)
	selectArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying approved comments: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating comments: %w", err)
	}

	if comments == nil {
		comments = []*Comment{}
	}

	return comments, total, nil
}

func (r *CommentRepository) FindAll(ctx context.Context, params QueryParams) ([]*Comment, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if params.PostID != "" {
		where += " AND post_id = ?"
		args = append(args, params.PostID)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM comments " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting comments: %w", err)
	}

	orderBy := "ORDER BY created_at DESC"
	if params.Sort == "created_at" {
		orderBy = "ORDER BY created_at ASC"
	}

	offset := (params.Page - 1) * params.PerPage
	limit := params.PerPage

	selectQuery := fmt.Sprintf(
		`SELECT id, display_id, post_id, author_name, body, approved,
		 ip_address, user_agent, turnstile_verified, created_at, updated_at
		 FROM comments %s %s LIMIT ? OFFSET ?`,
		where, orderBy,
	)
	selectArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying comments: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating comments: %w", err)
	}

	if comments == nil {
		comments = []*Comment{}
	}

	return comments, total, nil
}

func (r *CommentRepository) UpdateApproved(ctx context.Context, id int64, approved bool) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE comments SET approved = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		approved, id,
	)
	if err != nil {
		return fmt.Errorf("updating approved for comment %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM comments WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("deleting comment %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}
