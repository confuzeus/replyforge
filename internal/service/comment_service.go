package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/confuzeus/replyforge/internal/middleware"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	"github.com/confuzeus/replyforge/internal/sanitizer"
)

type turnstileVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

type ServiceDependencies struct {
	Repository   *repository.CommentRepository
	DisplayIDGen *model.DisplayIDGenerator
	Turnstile    turnstileVerifier
	Sanitizer    *sanitizer.Sanitizer
	Logger       *slog.Logger
}

type CommentService struct {
	repo         *repository.CommentRepository
	displayIDGen *model.DisplayIDGenerator
	turnstile    turnstileVerifier
	sanitizer    *sanitizer.Sanitizer
	mu           sync.Mutex
	logger       *slog.Logger
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

type ServiceError struct {
	Code    string
	Message string
	Err     error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

func NewCommentService(deps ServiceDependencies) *CommentService {
	return &CommentService{
		repo:         deps.Repository,
		displayIDGen: deps.DisplayIDGen,
		turnstile:    deps.Turnstile,
		sanitizer:    deps.Sanitizer,
		logger:       deps.Logger,
	}
}

func (s *CommentService) Create(ctx context.Context, input CreateInput) (*model.CommentResponse, error) {
	authorName := s.sanitizer.Sanitize(input.AuthorName)
	body := s.sanitizer.Sanitize(input.Body)

	ok, err := s.turnstile.Verify(ctx, input.TurnstileToken, input.ClientIP)
	middleware.TurnstileVerificationsTotal.Add(1)
	if err != nil {
		middleware.TurnstileFailedTotal.Add(1)
		s.logger.Error("turnstile verification failed", "error", err)
		return nil, &ServiceError{Code: "TURNSTILE_FAILED", Message: "Turnstile verification failed", Err: err}
	}
	if !ok {
		middleware.TurnstileFailedTotal.Add(1)
		s.logger.Warn("turnstile verification unsuccessful", "client_ip", input.ClientIP)
		return nil, &ServiceError{Code: "TURNSTILE_FAILED", Message: "Turnstile verification unsuccessful"}
	}

	comment := &repository.Comment{
		PostID:            input.PostID,
		AuthorName:        authorName,
		Body:              body,
		Approved:          false,
		IPAddress:         input.ClientIP,
		UserAgent:         input.UserAgent,
		TurnstileVerified: true,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := s.repo.Insert(ctx, comment)
	if err != nil {
		s.logger.Error("failed to insert comment", "error", err)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to create comment", Err: err}
	}

	displayID, err := s.displayIDGen.Generate(id)
	if err != nil {
		s.logger.Error("failed to generate display id", "error", err, "comment_id", id)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to generate display ID", Err: err}
	}

	if err := s.repo.UpdateDisplayID(ctx, id, displayID); err != nil {
		s.logger.Error("failed to update display id", "error", err, "comment_id", id)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to update display ID", Err: err}
	}

	created, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to retrieve created comment", "error", err, "comment_id", id)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to retrieve created comment", Err: err}
	}

	s.logger.Info("comment created",
		"request_id", middleware.RequestIDFromContext(ctx),
		"id", id,
		"display_id", displayID,
		"post_id", input.PostID,
		"author_name", authorName,
	)

	middleware.CommentsCreatedTotal.Add(1)

	return mapToResponse(created), nil
}

func (s *CommentService) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.PerPage > 100 {
		params.PerPage = 100
	}

	repoSort := ""
	if params.Sort == "created_at" {
		repoSort = "created_at"
	}

	comments, total, err := s.repo.FindApproved(ctx, repository.QueryParams{
		PostID:  params.PostID,
		Page:    params.Page,
		PerPage: params.PerPage,
		Sort:    repoSort,
	})
	if err != nil {
		s.logger.Error("failed to list comments", "error", err)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to list comments", Err: err}
	}

	responses := make([]*model.CommentResponse, len(comments))
	for i, c := range comments {
		responses[i] = mapToResponse(c)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PerPage)))

	return &ListResult{
		Comments:   responses,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (s *CommentService) GetByID(ctx context.Context, id int64) (*model.CommentResponse, error) {
	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ServiceError{Code: "NOT_FOUND", Message: "Comment not found", Err: err}
	}
	if !comment.Approved {
		return nil, &ServiceError{Code: "NOT_FOUND", Message: "Comment not found"}
	}

	return mapToResponse(comment), nil
}

func (s *CommentService) ListAll(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.PerPage > 100 {
		params.PerPage = 100
	}

	repoSort := ""
	if params.Sort == "created_at" {
		repoSort = "created_at"
	}

	comments, total, err := s.repo.FindAll(ctx, repository.QueryParams{
		PostID:  params.PostID,
		Page:    params.Page,
		PerPage: params.PerPage,
		Sort:    repoSort,
	})
	if err != nil {
		s.logger.Error("failed to list all comments", "error", err)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to list comments", Err: err}
	}

	responses := make([]*model.CommentResponse, len(comments))
	for i, c := range comments {
		responses[i] = mapToResponse(c)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PerPage)))

	return &ListResult{
		Comments:   responses,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (s *CommentService) GetByIDUnrestricted(ctx context.Context, id int64) (*model.CommentResponse, error) {
	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ServiceError{Code: "NOT_FOUND", Message: "Comment not found", Err: err}
	}
	return mapToResponse(comment), nil
}

func (s *CommentService) ToggleApproval(ctx context.Context, id int64) (*model.CommentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ServiceError{Code: "NOT_FOUND", Message: "Comment not found", Err: err}
	}

	newApproved := !comment.Approved
	if err := s.repo.UpdateApproved(ctx, id, newApproved); err != nil {
		s.logger.Error("failed to toggle approval", "error", err, "comment_id", id)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to toggle approval", Err: err}
	}

	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to retrieve updated comment", "error", err, "comment_id", id)
		return nil, &ServiceError{Code: "INTERNAL_ERROR", Message: "Failed to retrieve updated comment", Err: err}
	}

	s.logger.Info("comment approval toggled",
		"comment_id", id,
		"new_approved", newApproved,
	)

	return mapToResponse(updated), nil
}

func (s *CommentService) Delete(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.repo.Delete(ctx, id); err != nil {
		return &ServiceError{Code: "NOT_FOUND", Message: "Comment not found", Err: err}
	}

	s.logger.Info("comment deleted",
		"comment_id", id,
	)

	return nil
}

func mapToResponse(c *repository.Comment) *model.CommentResponse {
	return &model.CommentResponse{
		ID:         c.ID,
		DisplayID:  c.DisplayID,
		PostID:     c.PostID,
		AuthorName: c.AuthorName,
		Body:       c.Body,
		Approved:   c.Approved,
		CreatedAt:  c.CreatedAt,
	}
}
