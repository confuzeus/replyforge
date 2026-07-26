package model

import "time"

type CommentResponse struct {
	ID         int64     `json:"id"`
	DisplayID  string    `json:"display_id"`
	PostID     string    `json:"post_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	Approved   bool      `json:"approved"`
	CreatedAt  time.Time `json:"created_at"`
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
