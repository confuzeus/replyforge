package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/confuzeus/replyforge/internal/auth"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAdminHandler(t *testing.T, db *sql.DB, passwordHash string) *AdminHandler {
	t.Helper()
	svc := newTestService(t, db, true)
	tmpl := template.Must(template.New("admin").Parse("<html></html>"))
	sessionManager := auth.NewSessionManager(1*time.Hour, true)
	t.Cleanup(sessionManager.Stop)
	return NewAdminHandler(AdminHandlerDependencies{
		Service:        svc,
		AdminPage:      tmpl,
		PasswordHash:   passwordHash,
		SessionManager: sessionManager,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
}

func addAuthCookie(t *testing.T, h *AdminHandler, r *http.Request) {
	t.Helper()
	token, err := h.sessionManager.CreateSession()
	require.NoError(t, err)
	r.Header.Set("Cookie", "admin_session="+token)
}

func seedUnapproved(t *testing.T, db *sql.DB) *repository.Comment {
	t.Helper()
	repo := repository.NewCommentRepository(db)
	c := &repository.Comment{
		PostID:          "post-1",
		AuthorName:      "Unapproved",
		Body:            "Not approved yet",
		IPAddress:       "127.0.0.1",
		UserAgent:       "test-agent",
		CaptchaVerified: true,
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

func TestAdminServePage(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	handler.ServePage(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html>")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestAdminLogin_Success(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	body := `{"password":"admin-pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Authenticated", resp["message"])

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)
	assert.NotEmpty(t, sessionCookie.Value)
	assert.True(t, sessionCookie.HttpOnly)
	assert.True(t, sessionCookie.Secure)
	assert.Equal(t, "/api/v1/admin", sessionCookie.Path)

	assert.True(t, handler.sessionManager.ValidateSession(sessionCookie.Value))
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	body := `{"password":"wrong-pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminLogin_NoPasswordConfigured(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	body := `{"password":"anything"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminLogin_InvalidJSON(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminLogout(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	token, err := handler.sessionManager.CreateSession()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logout", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: token})
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Logged out", resp["message"])

	assert.False(t, handler.sessionManager.ValidateSession(token))

	cookies := rec.Result().Cookies()
	var clearCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "admin_session" {
			clearCookie = c
			break
		}
	}
	require.NotNil(t, clearCookie)
	assert.Equal(t, "", clearCookie.Value)
	assert.Equal(t, -1, clearCookie.MaxAge)
}

func TestAdminListAll_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	seedApprovedComment(t, db, "post-1", "Alice", "First")
	seedUnapproved(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ListAll(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 2, resp.Pagination.Total)
}

func TestAdminListAll_FilterByPost(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	seedApprovedComment(t, db, "post-a", "Alice", "Comment")
	seedUnapproved(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments?post_id=post-a", nil)
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ListAll(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Len(t, resp.Data, 1)
}

func TestAdminListAll_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ListAll(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Pagination.Total)
}

func TestAdminListAll_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	seedApprovedComment(t, db, "post-1", "Alice", "First")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	rec := httptest.NewRecorder()

	handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.ListAll)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminGetByID_Approved(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	c := seedApprovedComment(t, db, "post-1", "Alice", "Hello")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments/"+strconv.FormatInt(c.ID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	var data model.CommentResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, c.ID, data.ID)
	assert.True(t, data.Approved)
}

func TestAdminGetByID_Unapproved(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	c := seedUnapproved(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments/"+strconv.FormatInt(c.ID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	var data model.CommentResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, c.ID, data.ID)
	assert.False(t, data.Approved)
}

func TestAdminGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments/99999", nil)
	req.SetPathValue("id", "99999")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminGetByID_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments/abc", nil)
	req.SetPathValue("id", "abc")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminGetByID_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.GetByID)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminToggleApproval_Success(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	c := seedApprovedComment(t, db, "post-1", "Alice", "Toggle me")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/"+strconv.FormatInt(c.ID, 10)+"/toggle-approval", nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ToggleApproval(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["message"], "rejected")
}

func TestAdminToggleApproval_NotFound(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/99999/toggle-approval", nil)
	req.SetPathValue("id", "99999")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ToggleApproval(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminToggleApproval_InvalidID(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/abc/toggle-approval", nil)
	req.SetPathValue("id", "abc")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.ToggleApproval(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminToggleApproval_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/comments/1/toggle-approval", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.ToggleApproval)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminDelete_Success(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	c := seedApprovedComment(t, db, "post-1", "Alice", "Delete me")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/"+strconv.FormatInt(c.ID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Comment deleted", resp["message"])
}

func TestAdminDelete_NotFound(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/99999", nil)
	req.SetPathValue("id", "99999")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminDelete_InvalidID(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/abc", nil)
	req.SetPathValue("id", "abc")
	addAuthCookie(t, handler, req)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminDelete_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.Delete)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminLogin_CreatesUsableSession(t *testing.T) {
	hash, err := auth.GenerateHash("admin-pass")
	require.NoError(t, err)

	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, hash)

	seedApprovedComment(t, db, "post-1", "Alice", "Hello")

	body := `{"password":"admin-pass"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	handler.Login(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	var sessionCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	listReq.Header.Set("Cookie", "admin_session="+sessionCookie.Value)
	listRec := httptest.NewRecorder()

	middleware := handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.ListAll))
	middleware.ServeHTTP(listRec, listReq)

	assert.Equal(t, http.StatusOK, listRec.Code)

	var resp model.ListCommentsResponse
	require.NoError(t, json.NewDecoder(listRec.Body).Decode(&resp))
	assert.Len(t, resp.Data, 1)
}

func TestAdminSession_DeletedIsRejected(t *testing.T) {
	db := setupTestDB(t)
	handler := newTestAdminHandler(t, db, "")

	token, err := handler.sessionManager.CreateSession()
	require.NoError(t, err)

	handler.sessionManager.DeleteSession(token)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/comments", nil)
	req.Header.Set("Cookie", "admin_session="+token)
	rec := httptest.NewRecorder()

	handler.sessionManager.AuthMiddleware(http.HandlerFunc(handler.ListAll)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
