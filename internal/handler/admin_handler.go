package handler

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/confuzeus/replyforge/internal/auth"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/service"
)

type AdminHandler struct {
	service        *service.CommentService
	adminPage      *template.Template
	passwordHash   string
	sessionManager *auth.SessionManager
	logger         *slog.Logger
}

type AdminHandlerDependencies struct {
	Service        *service.CommentService
	AdminPage      *template.Template
	PasswordHash   string
	SessionManager *auth.SessionManager
	Logger         *slog.Logger
}

func NewAdminHandler(deps AdminHandlerDependencies) *AdminHandler {
	return &AdminHandler{
		service:        deps.Service,
		adminPage:      deps.AdminPage,
		passwordHash:   deps.PasswordHash,
		sessionManager: deps.SessionManager,
		logger:         deps.Logger,
	}
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	authMW := h.sessionManager.AuthMiddleware

	mux.HandleFunc("GET /admin", h.ServePage)
	mux.HandleFunc("POST /api/v1/admin/login", h.Login)
	mux.Handle("POST /api/v1/admin/logout", authMW(http.HandlerFunc(h.Logout)))
	mux.Handle("GET /api/v1/admin/verify", authMW(http.HandlerFunc(h.Verify)))
	mux.Handle("GET /api/v1/admin/comments", authMW(http.HandlerFunc(h.ListAll)))
	mux.Handle("GET /api/v1/admin/comments/{id}", authMW(http.HandlerFunc(h.GetByID)))
	mux.Handle("POST /api/v1/admin/comments/{id}/toggle-approval", authMW(http.HandlerFunc(h.ToggleApproval)))
	mux.Handle("DELETE /api/v1/admin/comments/{id}", authMW(http.HandlerFunc(h.Delete)))
}

func (h *AdminHandler) ServePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.adminPage.Execute(w, nil)
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.passwordHash == "" {
		writeError(w, http.StatusNotImplemented, "NOT_CONFIGURED", "Admin password is not configured", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body", nil)
		return
	}

	ok, err := auth.VerifyPassword(req.Password, h.passwordHash)
	if err != nil {
		h.logger.Error("password verification failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Authentication system error", nil)
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid password", nil)
		return
	}

	token, err := h.sessionManager.CreateSession()
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create session", nil)
		return
	}

	h.sessionManager.SetCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Authenticated",
	})
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := h.sessionManager.GetToken(r)
	if token != "" {
		h.sessionManager.DeleteSession(token)
	}
	h.sessionManager.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Logged out",
	})
}

func (h *AdminHandler) Verify(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "ok",
	})
}

func (h *AdminHandler) ListAll(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.ListAll(r.Context(), params)
	if err != nil {
		h.writeError(w, err)
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

func (h *AdminHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID format", nil)
		return
	}

	comment, err := h.service.GetByIDUnrestricted(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": comment})
}

func (h *AdminHandler) ToggleApproval(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID format", nil)
		return
	}

	comment, err := h.service.ToggleApproval(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}

	status := "rejected"
	if comment.Approved {
		status = "approved"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Comment " + status,
		"data":    comment,
	})
}

func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID format", nil)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Comment deleted",
	})
}

func (h *AdminHandler) writeError(w http.ResponseWriter, err error) {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		status, code := serviceErrorToHTTP(svcErr)
		writeError(w, status, code, svcErr.Message, nil)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
}
