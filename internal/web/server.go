package web

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/domain"
	"git.sr.ht/~jakintosh/consent/pkg/client"
)

// AuthConfig configures authentication for the server.
// When nil is passed to ServerOptions, the server runs without auth capability.
type AuthConfig struct {
	// Verifier validates access tokens
	Verifier client.Verifier

	// LoginURL is where the login button should send users
	LoginURL string

	// LogoutURL is where the logout button should send users
	LogoutURL string

	// Routes are mode-specific handlers to register (e.g., /dev/login, /auth/callback)
	Routes map[string]http.HandlerFunc
}

// ServerOptions configures the web server
type ServerOptions struct {
	Auth AuthConfig // Required; Verifier must be non-nil
}

type Server struct {
	store        domain.Store
	router       *http.ServeMux
	presentation *Presentation
	auth         AuthConfig
}

func NewServer(store domain.Store, opts ServerOptions) (*Server, error) {
	if opts.Auth.Verifier == nil {
		return nil, errors.New("Auth.Verifier is required")
	}

	pres, err := NewPresentation()
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:        store,
		router:       http.NewServeMux(),
		presentation: pres,
		auth:         opts.Auth,
	}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Static Files
	fs := http.FileServer(http.Dir("internal/web/static"))
	s.router.Handle("/static/", http.StripPrefix("/static/", fs))

	// Auth routes (mode-specific: /dev/login, /dev/logout, /auth/callback, etc.)
	for path, handler := range s.auth.Routes {
		s.router.HandleFunc(path, handler)
	}

	// Page Routes
	s.router.HandleFunc("GET /{$}", s.handleIndex)

	// API/HTMX Routes
	s.router.HandleFunc("POST /categories", s.handleCreateCategory)
	s.router.HandleFunc("PATCH /categories/{id}", s.handleUpdateCategory)
	s.router.HandleFunc("GET /categories/{id}/details", s.handleGetCategoryDetails)
	s.router.HandleFunc("POST /categories/{id}/tasks", s.handleCreateTask)
	s.router.HandleFunc("PATCH /tasks/{id}", s.handleUpdateTask)
	s.router.HandleFunc("GET /tasks/{id}/details", s.handleGetTaskDetails)
	s.router.HandleFunc("POST /tasks/{id}/subtasks", s.handleCreateSubtask)
	s.router.HandleFunc("PATCH /subtasks/{id}", s.handleUpdateSubtask)
	s.router.HandleFunc("POST /categories/reorder", s.handleReorderCategories)
	s.router.HandleFunc("POST /tasks/reorder", s.handleReorderTasks)
	s.router.HandleFunc("GET /subtasks/{id}/details", s.handleGetSubtaskDetails)
	s.router.HandleFunc("POST /subtasks/reorder", s.handleReorderSubtasks)
	s.router.HandleFunc("DELETE /categories/{id}", s.handleDeleteCategory)
	s.router.HandleFunc("DELETE /tasks/{id}", s.handleDeleteTask)
	s.router.HandleFunc("DELETE /subtasks/{id}", s.handleDeleteSubtask)

	// Work Log Routes
	s.router.HandleFunc("POST /tasks/{id}/work-logs", s.handleCreateTaskWorkLog)
	s.router.HandleFunc("POST /subtasks/{id}/work-logs", s.handleCreateSubtaskWorkLog)
}

// getAuthContext attempts to verify auth and returns context with CSRF token.
// Returns unauthenticated context if verification fails.
func (s *Server) getAuthContext(w http.ResponseWriter, r *http.Request) AuthContext {
	// Build base context with auth URLs
	ctx := AuthContext{
		IsAuthenticated: false,
		LoginURL:        s.auth.LoginURL,
		LogoutURL:       s.auth.LogoutURL,
	}

	accessToken, csrfToken, err := s.auth.Verifier.VerifyAuthorizationGetCSRF(w, r)
	if err != nil {
		return ctx
	}

	ctx.IsAuthenticated = true
	ctx.Handle = accessToken.Subject()
	ctx.CSRFToken = csrfToken
	return ctx
}

// requireAuth verifies auth and CSRF for destructive operations.
// Returns auth context and true if authorized, writes error response if not.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (AuthContext, bool) {
	// Get CSRF from request (form value or query param)
	csrf := r.FormValue("csrf")
	if csrf == "" {
		csrf = r.URL.Query().Get("csrf")
	}

	accessToken, csrfToken, err := s.auth.Verifier.VerifyAuthorizationCheckCSRF(w, r, csrf)
	if err == client.ErrCSRFInvalid {
		http.Error(w, "CSRF validation failed", http.StatusForbidden)
		return AuthContext{}, false
	}
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return AuthContext{}, false
	}

	return AuthContext{
		IsAuthenticated: true,
		Handle:          accessToken.Subject(),
		CSRFToken:       csrfToken,
		LoginURL:        s.auth.LoginURL,
		LogoutURL:       s.auth.LogoutURL,
	}, true
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)

	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.presentation.RenderIndex(w, catViews, auth); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	cat, err := s.store.AddCategory("New Category")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	catView := NewCategoryView(cat, false, auth)
	if err := s.presentation.RenderCategory(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.presentation.RenderSlideoverWithDetails(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	cat, err := s.store.GetCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if name := r.FormValue("name"); name != "" {
		cat.Name = name
	}
	if desc := r.FormValue("description"); desc != "" {
		cat.Description = desc
	}

	cat, err = s.store.UpdateCategory(cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Render OOB updates for category name (in header)
	catView := NewCategoryView(cat, true, auth)
	if err := s.presentation.RenderCategory(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetCategoryDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	cat, err := s.store.GetCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Fetch work logs for category
	workLogs, err := s.store.GetWorkLogsForCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cat.WorkLogs = workLogs

	if ctx.IsHTMX {
		if err := s.presentation.RenderCategoryDetails(w, NewCategoryView(cat, false, auth)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, auth, NewCategoryView(cat, false, auth)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	catID := r.PathValue("id")

	task, err := s.store.AddTask(catID, "New Task")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.store.GetCategory(catID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())

	taskView := NewTaskView(task, false, auth)
	if err := s.presentation.RenderSlideoverWithDetails(w, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if name := r.FormValue("name"); name != "" {
		task.Name = name
	}
	if desc := r.FormValue("description"); desc != "" {
		task.Description = desc
	}
	if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			task.Completion = val
		}
	}

	task, err = s.store.UpdateTask(task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.store.GetCategory(task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleGetSubtaskDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	sub, err := s.store.GetSubtask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Fetch work logs for subtask
	workLogs, err := s.store.GetWorkLogsForSubtask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sub.WorkLogs = workLogs
	subtaskView := NewSubtaskView(sub, false, auth)

	if ctx.IsHTMX {
		if err := s.presentation.RenderSubtaskDetails(w, subtaskView); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, auth, subtaskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetTaskDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Fetch work logs for task
	workLogs, err := s.store.GetWorkLogsForTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	task.WorkLogs = workLogs
	taskView := NewTaskView(task, false, auth)

	if ctx.IsHTMX {
		if err := s.presentation.RenderTaskDetails(w, taskView); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, auth, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	taskID := r.PathValue("id")

	sub, err := s.store.AddSubtask(taskID, "New Subtask")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.store.GetCategory(sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())

	subtaskView := NewSubtaskView(sub, false, auth)
	if err := s.presentation.RenderSlideoverWithDetails(w, subtaskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateSubtask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	sub, err := s.store.GetSubtask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if name := r.FormValue("name"); name != "" {
		sub.Name = name
	}
	if desc := r.FormValue("description"); desc != "" {
		sub.Description = desc
	}
	if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			sub.Completion = val
		}
	}

	sub, err = s.store.UpdateSubtask(sub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.store.GetCategory(sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	ctx := parseRequestContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ids := r.Form["id"]
	if len(ids) == 0 {
		return // Nothing to do
	}

	if err := s.store.ReorderCategories(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	ctx := parseRequestContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	catID := r.FormValue("category_id")
	ids := r.Form["id"]
	if catID == "" || len(ids) == 0 {
		return // Nothing to do
	}

	if err := s.store.ReorderTasks(catID, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderSubtasks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	ctx := parseRequestContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	taskID := r.FormValue("task_id")
	ids := r.Form["id"]

	if err := s.store.ReorderSubtasks(taskID, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	if _, err := s.store.DeleteCategory(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := s.presentation.RenderCategoryDeleteOOB(w, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.DeleteTask(id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.store.GetCategory(task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.presentation.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleDeleteSubtask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	sub, err := s.store.DeleteSubtask(id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.store.GetCategory(sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.presentation.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.presentation.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleCreateTaskWorkLog(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	taskID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	hoursWorked, err := strconv.ParseFloat(r.FormValue("hours_worked"), 64)
	if err != nil {
		http.Error(w, "Invalid hours_worked value", http.StatusBadRequest)
		return
	}

	completionEstimate, err := strconv.Atoi(r.FormValue("completion_estimate"))
	if err != nil {
		http.Error(w, "Invalid completion_estimate value", http.StatusBadRequest)
		return
	}

	workDescription := r.FormValue("work_description")

	// Parse optional custom timestamp
	var customTime *time.Time
	if r.FormValue("use_custom_time") == "on" {
		if ct := r.FormValue("custom_time"); ct != "" {
			if parsed, err := time.ParseInLocation("2006-01-02T15:04", ct, time.Local); err == nil {
				customTime = &parsed
			}
		}
	}

	workLog, err := s.store.AddWorkLogForTask(taskID, hoursWorked, workDescription, completionEstimate, customTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.store.GetCategory(workLog.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.presentation.RenderCategoryOOB(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleCreateSubtaskWorkLog(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	subtaskID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	hoursWorked, err := strconv.ParseFloat(r.FormValue("hours_worked"), 64)
	if err != nil {
		http.Error(w, "Invalid hours_worked value", http.StatusBadRequest)
		return
	}

	completionEstimate, err := strconv.Atoi(r.FormValue("completion_estimate"))
	if err != nil {
		http.Error(w, "Invalid completion_estimate value", http.StatusBadRequest)
		return
	}

	workDescription := r.FormValue("work_description")

	// Parse optional custom timestamp
	var customTime *time.Time
	if r.FormValue("use_custom_time") == "on" {
		if ct := r.FormValue("custom_time"); ct != "" {
			if parsed, err := time.ParseInLocation("2006-01-02T15:04", ct, time.Local); err == nil {
				customTime = &parsed
			}
		}
	}

	workLog, err := s.store.AddWorkLogForSubtask(subtaskID, hoursWorked, workDescription, completionEstimate, customTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.store.GetCategory(workLog.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.presentation.RenderCategoryOOB(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
