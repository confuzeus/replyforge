package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/confuzeus/replyforge/internal/middleware"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/service"
)

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

func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	var req model.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body", nil)
		return
	}

	if verr := req.Validate(); verr != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The request contains invalid parameters", verr.Fields)
		return
	}

	clientIP := middleware.ExtractClientIP(r)

	input := service.CreateInput{
		PostID:         req.PostID,
		AuthorName:     req.AuthorName,
		Body:           req.Body,
		TurnstileToken: req.TurnstileToken,
		ClientIP:       clientIP,
		UserAgent:      r.UserAgent(),
	}

	comment, err := h.service.Create(r.Context(), input)
	if err != nil {
		var svcErr *service.ServiceError
		if errors.As(err, &svcErr) {
			status, code := serviceErrorToHTTP(svcErr)
			writeError(w, status, code, svcErr.Message, nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": comment})
}

func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	params := service.ListParams{
		PostID: query.Get("post_id"),
		Sort:   query.Get("sort"),
	}

	if pageStr := query.Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			params.Page = page
		}
	}
	if perPageStr := query.Get("per_page"); perPageStr != "" {
		if perPage, err := strconv.Atoi(perPageStr); err == nil {
			params.PerPage = perPage
		}
	}

	result, err := h.service.List(r.Context(), params)
	if err != nil {
		var svcErr *service.ServiceError
		if errors.As(err, &svcErr) {
			status, code := serviceErrorToHTTP(svcErr)
			writeError(w, status, code, svcErr.Message, nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
		return
	}

	resp := model.ListCommentsResponse{
		Data: result.Comments,
		Pagination: model.PaginationMeta{
			Page:       result.Page,
			PerPage:    result.PerPage,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *CommentHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID format", nil)
		return
	}

	comment, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		var svcErr *service.ServiceError
		if errors.As(err, &svcErr) {
			status, code := serviceErrorToHTTP(svcErr)
			writeError(w, status, code, svcErr.Message, nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": comment})
}

func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, statusCode int, code, message string, details []model.FieldError) {
	if code == "VALIDATION_ERROR" {
		middleware.ValidationErrorsTotal.Add(1)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	resp := model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(resp)
}

func serviceErrorToHTTP(svcErr *service.ServiceError) (int, string) {
	switch svcErr.Code {
	case "VALIDATION_ERROR":
		return http.StatusBadRequest, svcErr.Code
	case "NOT_FOUND":
		return http.StatusNotFound, svcErr.Code
	case "TURNSTILE_FAILED":
		return http.StatusForbidden, svcErr.Code
	case "INTERNAL_ERROR":
		return http.StatusInternalServerError, svcErr.Code
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
