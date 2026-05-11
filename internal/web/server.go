package web

import (
	"bytes"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
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

	// ProfileFetcher fetches scoped profile data from Consent. When unset, the
	// token subject is used as the local development handle.
	ProfileFetcher interface {
		FetchUserInfo(accessToken string) (*client.UserInfo, error)
	}

	// ProfileRefreshInterval controls how often cached Consent profile data is refreshed.
	ProfileRefreshInterval time.Duration

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
	store    domain.Store
	router   *http.ServeMux
	renderer *Renderer
	auth     AuthConfig
}

func NewServer(store domain.Store, opts ServerOptions) (*Server, error) {
	if opts.Auth.Verifier == nil {
		return nil, errors.New("Auth.Verifier is required")
	}

	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:    store,
		router:   http.NewServeMux(),
		renderer: renderer,
		auth:     opts.Auth,
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
	s.router.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Auth routes (mode-specific: /dev/login, /dev/logout, /auth/callback, etc.)
	for path, handler := range s.auth.Routes {
		s.router.HandleFunc(path, handler)
	}

	// Page Routes
	s.router.HandleFunc("GET /{$}", s.handleIndex)

	s.router.HandleFunc("/", s.handleTenantRoute)
}

func (s *Server) handleTenantRoute(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		http.NotFound(w, r)
		return
	}
	switch segments[0] {
	case "static", "auth", "dev", ".well-known":
		http.NotFound(w, r)
		return
	}

	r.SetPathValue("handle", segments[0])
	if len(segments) == 1 && r.Method == http.MethodGet {
		s.handleTenantIndex(w, r)
		return
	}

	if len(segments) < 2 {
		http.NotFound(w, r)
		return
	}

	switch segments[1] {
	case "categories":
		s.dispatchCategoryRoute(w, r, segments)
	case "tasks":
		s.dispatchTaskRoute(w, r, segments)
	case "subtasks":
		s.dispatchSubtaskRoute(w, r, segments)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) dispatchCategoryRoute(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 2 && r.Method == http.MethodPost {
		s.handleCreateCategory(w, r)
		return
	}
	if len(segments) == 3 && segments[2] == "reorder" && r.Method == http.MethodPost {
		s.handleReorderCategories(w, r)
		return
	}
	if len(segments) >= 3 {
		r.SetPathValue("id", segments[2])
	}
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodPatch:
			s.handleUpdateCategory(w, r)
		case http.MethodDelete:
			s.handleDeleteCategory(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(segments) == 4 && segments[3] == "details" && r.Method == http.MethodGet {
		s.handleGetCategoryDetails(w, r)
		return
	}
	if len(segments) == 4 && segments[3] == "tasks" && r.Method == http.MethodPost {
		s.handleCreateTask(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) dispatchTaskRoute(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 3 && segments[2] == "reorder" && r.Method == http.MethodPost {
		s.handleReorderTasks(w, r)
		return
	}
	if len(segments) >= 3 {
		r.SetPathValue("id", segments[2])
	}
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodPatch:
			s.handleUpdateTask(w, r)
		case http.MethodDelete:
			s.handleDeleteTask(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(segments) == 4 && segments[3] == "details" && r.Method == http.MethodGet {
		s.handleGetTaskDetails(w, r)
		return
	}
	if len(segments) == 4 && segments[3] == "subtasks" && r.Method == http.MethodPost {
		s.handleCreateSubtask(w, r)
		return
	}
	if len(segments) == 4 && segments[3] == "work-logs" && r.Method == http.MethodPost {
		s.handleCreateTaskWorkLog(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) dispatchSubtaskRoute(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 3 && segments[2] == "reorder" && r.Method == http.MethodPost {
		s.handleReorderSubtasks(w, r)
		return
	}
	if len(segments) >= 3 {
		r.SetPathValue("id", segments[2])
	}
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodPatch:
			s.handleUpdateSubtask(w, r)
		case http.MethodDelete:
			s.handleDeleteSubtask(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(segments) == 4 && segments[3] == "details" && r.Method == http.MethodGet {
		s.handleGetSubtaskDetails(w, r)
		return
	}
	if len(segments) == 4 && segments[3] == "work-logs" && r.Method == http.MethodPost {
		s.handleCreateSubtaskWorkLog(w, r)
		return
	}
	http.NotFound(w, r)
}

// getAuthContext attempts to verify auth and returns context with CSRF token.
// Returns unauthenticated context if verification fails.
func (s *Server) getAuthContext(w http.ResponseWriter, r *http.Request) AuthContext {
	// Build base context with auth URLs
	ctx := AuthContext{
		IsAuthenticated: false,
		LoginURL:        s.loginURL(r),
		LogoutURL:       s.auth.LogoutURL,
	}

	accessToken, csrfToken, err := s.auth.Verifier.VerifyAuthorizationGetCSRF(w, r)
	if err != nil {
		return ctx
	}

	ctx.IsAuthenticated = true
	ctx.Subject = accessToken.Subject()
	ctx.CSRFToken = csrfToken
	ctx.LoginURL = s.loginURL(r)
	account, err := s.accountForToken(accessToken)
	if err == nil && account != nil {
		ctx.AccountID = account.ID
		ctx.Handle = account.Handle
	} else {
		log.Printf("failed to resolve authenticated account: %v", err)
		ctx.Handle = accessToken.Subject()
	}
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

	auth := AuthContext{
		IsAuthenticated: true,
		Subject:         accessToken.Subject(),
		CSRFToken:       csrfToken,
		LoginURL:        s.loginURL(r),
		LogoutURL:       s.auth.LogoutURL,
	}
	account, err := s.accountForToken(accessToken)
	if err != nil {
		http.Error(w, "Unable to resolve account", http.StatusUnauthorized)
		return AuthContext{}, false
	}
	auth.AccountID = account.ID
	auth.Handle = account.Handle
	return auth, true
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	if auth.IsAuthenticated && auth.Handle != "" {
		http.Redirect(w, r, "/"+url.PathEscape(auth.Handle)+"/", http.StatusSeeOther)
		return
	}

	if err := s.renderer.RenderIndex(w, nil, auth); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTenantIndex(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}

	cats, err := s.store.GetCategories(account.ID)
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	// Filter out non-public items for unauthenticated users or non-owner viewers.
	if !auth.CanWrite {
		cats = filterPublicCategories(cats)
	}

	// Convert to view models
	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndex(w, catViews, auth); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) loginURL(r *http.Request) string {
	if s.auth.LoginURL == "" {
		return ""
	}
	loginURL, err := url.Parse(s.auth.LoginURL)
	if err != nil {
		return s.auth.LoginURL
	}
	q := loginURL.Query()
	if r.URL.RequestURI() != "/" {
		q.Set("return_to", r.URL.RequestURI())
	}
	loginURL.RawQuery = q.Encode()
	return loginURL.String()
}

func (s *Server) accountForToken(accessToken *client.AccessToken) (*domain.Account, error) {
	subject := accessToken.Subject()
	account, err := s.store.GetAccountBySubject(subject)
	if err == nil && !s.shouldRefreshProfile(account) {
		return account, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	handle := subject
	refreshedAt := time.Now()
	if s.auth.ProfileFetcher != nil {
		userInfo, fetchErr := s.auth.ProfileFetcher.FetchUserInfo(accessToken.Encoded())
		if fetchErr != nil {
			if account != nil {
				log.Printf("failed to refresh consent profile for %s: %v", subject, fetchErr)
				return account, nil
			}
			return nil, fetchErr
		}
		if userInfo.Profile == nil || userInfo.Profile.Handle == "" {
			return nil, errors.New("consent profile missing handle")
		}
		handle = userInfo.Profile.Handle
	}

	account, err = s.store.UpsertAccount(subject, handle, refreshedAt)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Server) shouldRefreshProfile(account *domain.Account) bool {
	interval := s.auth.ProfileRefreshInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return time.Since(account.ProfileRefreshedAt) >= interval
}

func (s *Server) resolveTenant(w http.ResponseWriter, r *http.Request, auth AuthContext, requireOwner bool) (*domain.Account, AuthContext, bool) {
	handle := r.PathValue("handle")
	account, err := s.store.GetAccountByHandle(handle)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return nil, auth, false
	}

	auth.BasePath = "/" + url.PathEscape(account.Handle)
	auth.CanWrite = auth.IsAuthenticated && auth.AccountID == account.ID
	if requireOwner && !auth.CanWrite {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, auth, false
	}
	return account, auth, true
}

// filterPublicCategories removes non-public categories, tasks, and subtasks
func filterPublicCategories(cats []*domain.Category) []*domain.Category {
	var result []*domain.Category
	for _, c := range cats {
		if !c.Public {
			continue
		}
		// Filter tasks within public category
		var publicTasks []*domain.Task
		for _, t := range c.Tasks {
			if !t.Public {
				continue
			}
			// Filter subtasks within public task
			var publicSubs []*domain.Subtask
			for _, s := range t.Subtasks {
				if s.Public {
					publicSubs = append(publicSubs, s)
				}
			}
			t.Subtasks = publicSubs
			publicTasks = append(publicTasks, t)
		}
		c.Tasks = publicTasks
		result = append(result, c)
	}
	return result
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	cat, err := s.store.AddCategory(account.ID, "New Category")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	catView := NewCategoryView(cat, false, auth)
	if err := s.renderer.RenderCategory(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.renderer.RenderSlideoverWithDetails(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	cat, err := s.store.GetCategory(account.ID, id)
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
	} else if desc := r.FormValue("description"); desc != "" {
		cat.Description = desc
	} else {
		// Public toggle form - checkbox sends "on" when checked, nothing when unchecked
		cat.Public = r.FormValue("public") == "on"
	}

	cat, err = s.store.UpdateCategory(account.ID, cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Render OOB updates for category
	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategory(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetCategoryDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	cat, err := s.store.GetCategory(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Private items are not accessible to unauthenticated users
	if !auth.CanWrite && !cat.Public {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Fetch work logs for category
	workLogs, err := s.store.GetWorkLogsForCategory(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cat.WorkLogs = workLogs

	if ctx.IsHTMX {
		if err := s.renderer.RenderCategoryDetails(w, NewCategoryView(cat, false, auth)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories(account.ID)
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	if !auth.CanWrite {
		cats = filterPublicCategories(cats)
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, NewCategoryView(cat, false, auth)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	catID := r.PathValue("id")

	task, err := s.store.AddTask(account.ID, catID, "New Task")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.store.GetCategory(account.ID, catID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())

	taskView := NewTaskView(task, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.GetTask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle form field updates - only one field per form submission
	if name := r.FormValue("name"); name != "" {
		task.Name = name
	} else if desc := r.FormValue("description"); desc != "" {
		task.Description = desc
	} else if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			task.Completion = val
		}
	} else {
		// Public toggle form
		task.Public = r.FormValue("public") == "on"
	}

	task, err = s.store.UpdateTask(account.ID, task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.store.GetCategory(account.ID, task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleGetSubtaskDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	sub, err := s.store.GetSubtask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Fetch work logs for subtask
	workLogs, err := s.store.GetWorkLogsForSubtask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sub.WorkLogs = workLogs

	// Private items are not accessible to unauthenticated users
	if !auth.CanWrite && (!sub.ParentPublic || !sub.Public) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	subtaskView := NewSubtaskView(sub, false, auth)

	if ctx.IsHTMX {
		if err := s.renderer.RenderSubtaskDetails(w, subtaskView); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories(account.ID)
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	if !auth.CanWrite {
		cats = filterPublicCategories(cats)
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, subtaskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetTaskDetails(w http.ResponseWriter, r *http.Request) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.GetTask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Fetch work logs for task
	workLogs, err := s.store.GetWorkLogsForTask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	task.WorkLogs = workLogs

	// Private items are not accessible to unauthenticated users
	if !auth.CanWrite && (!task.ParentPublic || !task.Public) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	taskView := NewTaskView(task, false, auth)

	if ctx.IsHTMX {
		if err := s.renderer.RenderTaskDetails(w, taskView); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.store.GetCategories(account.ID)
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	if !auth.CanWrite {
		cats = filterPublicCategories(cats)
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	taskID := r.PathValue("id")

	sub, err := s.store.AddSubtask(account.ID, taskID, "New Subtask")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.store.GetCategory(account.ID, sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())

	subtaskView := NewSubtaskView(sub, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, subtaskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateSubtask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	sub, err := s.store.GetSubtask(account.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle form field updates - only one field per form submission
	if name := r.FormValue("name"); name != "" {
		sub.Name = name
	} else if desc := r.FormValue("description"); desc != "" {
		sub.Description = desc
	} else if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			sub.Completion = val
		}
	} else {
		// Public toggle form
		sub.Public = r.FormValue("public") == "on"
	}

	sub, err = s.store.UpdateSubtask(account.ID, sub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.store.GetCategory(account.ID, sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
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

	if err := s.store.ReorderCategories(account.ID, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
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

	if err := s.store.ReorderTasks(account.ID, catID, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderSubtasks(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	taskID := r.FormValue("task_id")
	ids := r.Form["id"]

	if err := s.store.ReorderSubtasks(account.ID, taskID, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	if _, err := s.store.DeleteCategory(account.ID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	if err := s.renderer.RenderCategoryDeleteOOB(w, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.DeleteTask(account.ID, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.store.GetCategory(account.ID, task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderer.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
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
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	sub, err := s.store.DeleteSubtask(account.ID, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.store.GetCategory(account.ID, sub.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderer.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
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
	account, auth, ok := s.resolveTenant(w, r, auth, true)
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

	workLog, err := s.store.AddWorkLogForTask(account.ID, taskID, hoursWorked, workDescription, completionEstimate, customTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.store.GetCategory(account.ID, workLog.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch task with work logs and render slideover OOB update
	task, err := s.store.GetTask(account.ID, taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	taskWorkLogs, err := s.store.GetWorkLogsForTask(account.ID, taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	task.WorkLogs = taskWorkLogs

	taskView := NewTaskView(task, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateSubtaskWorkLog(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
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

	workLog, err := s.store.AddWorkLogForSubtask(account.ID, subtaskID, hoursWorked, workDescription, completionEstimate, customTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.store.GetCategory(account.ID, workLog.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch subtask with work logs and render slideover OOB update
	sub, err := s.store.GetSubtask(account.ID, subtaskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subWorkLogs, err := s.store.GetWorkLogsForSubtask(account.ID, subtaskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sub.WorkLogs = subWorkLogs

	subtaskView := NewSubtaskView(sub, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, subtaskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
