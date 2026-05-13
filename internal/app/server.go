package app

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/command-go/pkg/wire"
	"git.sr.ht/~jakintosh/compass/internal/service"
	"git.sr.ht/~jakintosh/consent/pkg/client"
)

// AuthConfig configures authentication for the rendered app.
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
}

// Options configures the rendered app.
type Options struct {
	Service *service.Service
	Auth    AuthConfig
}

type App struct {
	service  *service.Service
	renderer *Renderer
	auth     AuthConfig
}

func New(
	opts Options,
) (
	*App,
	error,
) {
	if opts.Auth.Verifier == nil {
		return nil, errors.New("Auth.Verifier is required")
	}
	if opts.Service == nil {
		return nil, errors.New("service is required")
	}

	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	app := &App{
		service:  opts.Service,
		renderer: renderer,
		auth:     opts.Auth,
	}
	return app, nil
}

func (s *App) Handler() http.Handler {
	root := http.NewServeMux()

	wire.Subrouter(root, "/static", s.buildStaticRouter())
	wire.Subrouter(root, "/", s.buildPageRouter())

	return root
}

func (s *App) buildStaticRouter() http.Handler {
	return http.FileServer(http.Dir("internal/app/static"))
}

func (s *App) buildPageRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("/", s.withTenantPath(s.buildTenantRouter()))

	return mux
}

func (s *App) buildTenantRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleTenantIndex)
	wire.Subrouter(mux, "/categories", s.buildCategoryRouter())
	wire.Subrouter(mux, "/projects", s.buildProjectRouter())
	wire.Subrouter(mux, "/tasks", s.buildTaskRouter())

	return mux
}

func (s *App) buildCategoryRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /", s.handleCreateCategory)
	mux.HandleFunc("POST /reorder", s.handleReorderCategories)
	mux.HandleFunc("PATCH /{id}", s.handleUpdateCategory)
	mux.HandleFunc("DELETE /{id}", s.handleDeleteCategory)
	mux.HandleFunc("GET /{id}/details", s.handleGetCategoryDetails)
	mux.HandleFunc("POST /{id}/projects", s.handleCreateProject)

	return mux
}

func (s *App) buildProjectRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /reorder", s.handleReorderProjects)
	mux.HandleFunc("PATCH /{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /{id}", s.handleDeleteProject)
	mux.HandleFunc("GET /{id}/details", s.handleGetProjectDetails)
	mux.HandleFunc("POST /{id}/tasks", s.handleCreateTask)
	mux.HandleFunc("POST /{id}/work-logs", s.handleCreateProjectWorkLog)

	return mux
}

func (s *App) buildTaskRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /reorder", s.handleReorderTasks)
	mux.HandleFunc("PATCH /{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /{id}", s.handleDeleteTask)
	mux.HandleFunc("GET /{id}/details", s.handleGetTaskDetails)
	mux.HandleFunc("POST /{id}/work-logs", s.handleCreateTaskWorkLog)

	return mux
}

func (s *App) withTenantPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			http.NotFound(w, r)
			return
		}

		handleSegment, tenantPath, _ := strings.Cut(path, "/")
		switch handleSegment {
		case "static", "auth", "dev", ".well-known":
			http.NotFound(w, r)
			return
		}

		handle, err := url.PathUnescape(handleSegment)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = tenantURLPath(tenantPath)
		r2.URL.RawPath = ""
		r2.SetPathValue("handle", handle)

		next.ServeHTTP(w, r2)
	})
}

func tenantURLPath(tenantPath string) string {
	if tenantPath == "" {
		return "/"
	}
	return "/" + tenantPath
}

// getAuthContext attempts to verify auth and returns context with CSRF token.
// Returns unauthenticated context if verification fails.
func (s *App) getAuthContext(
	w http.ResponseWriter,
	r *http.Request,
) AuthContext {
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
func (s *App) requireAuth(
	w http.ResponseWriter,
	r *http.Request,
) (
	AuthContext,
	bool,
) {
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

func (s *App) handleIndex(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth := s.getAuthContext(w, r)
	if auth.IsAuthenticated && auth.Handle != "" {
		http.Redirect(w, r, "/"+url.PathEscape(auth.Handle)+"/", http.StatusSeeOther)
		return
	}

	if err := s.renderer.RenderIndex(w, nil, auth); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleTenantIndex(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}

	cats, err := s.service.ListCategories(service.ListCategoriesInput{AccountID: account.ID, Viewer: viewerFromAuth(auth)})
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndex(w, catViews, auth); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) loginURL(
	r *http.Request,
) string {
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

func (s *App) accountForToken(
	accessToken *client.AccessToken,
) (
	*service.Account,
	error,
) {
	subject := accessToken.Subject()
	account, err := s.service.GetAccountBySubject(service.GetAccountBySubjectInput{Subject: subject})
	if err == nil && !s.shouldRefreshProfile(account) {
		return account, nil
	}
	if err != nil && !errors.Is(err, service.ErrNotFound) {
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

	account, err = s.service.UpsertAccount(service.UpsertAccountInput{Subject: subject, Handle: handle, RefreshedAt: refreshedAt})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *App) shouldRefreshProfile(
	account *service.Account,
) bool {
	interval := s.auth.ProfileRefreshInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return time.Since(account.ProfileRefreshedAt) >= interval
}

func (s *App) resolveTenant(
	w http.ResponseWriter,
	r *http.Request,
	auth AuthContext,
	requireOwner bool,
) (
	*service.Account,
	AuthContext,
	bool,
) {
	handle := r.PathValue("handle")
	account, err := s.service.GetAccountByHandle(service.GetAccountByHandleInput{Handle: handle})
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

func viewerFromAuth(
	auth AuthContext,
) service.Viewer {
	if auth.CanWrite {
		return service.OwnerViewer()
	}
	return service.PublicViewer()
}

func writeServiceError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, service.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *App) handleCreateCategory(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	cat, err := s.service.CreateCategory(service.CreateCategoryInput{AccountID: account.ID, Name: "New Category"})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	catView := NewCategoryView(cat, false, auth)
	if err := s.renderer.RenderCategory(w, catView); err != nil {
		writeServiceError(w, err)
		return
	}

	if err := s.renderer.RenderSlideoverWithDetails(w, catView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleUpdateCategory(
	w http.ResponseWriter,
	r *http.Request,
) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var update service.UpdateCategoryInput
	update.AccountID = account.ID
	update.ID = id
	if name := r.FormValue("name"); name != "" {
		update.Name = &name
	} else if desc := r.FormValue("description"); desc != "" {
		update.Description = &desc
	} else {
		// Public toggle form - checkbox sends "on" when checked, nothing when unchecked
		public := r.FormValue("public") == "on"
		update.Public = &public
	}

	cat, err := s.service.UpdateCategory(update)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Render OOB updates for category
	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategory(w, catView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleGetCategoryDetails(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	cat, err := s.service.GetCategoryWithWorkLogs(service.GetCategoryInput{AccountID: account.ID, ID: id, Viewer: viewerFromAuth(auth)})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if ctx.IsHTMX {
		if err := s.renderer.RenderCategoryDetails(w, NewCategoryView(cat, false, auth)); err != nil {
			writeServiceError(w, err)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.service.ListCategories(service.ListCategoriesInput{AccountID: account.ID, Viewer: viewerFromAuth(auth)})
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, NewCategoryView(cat, false, auth)); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleCreateProject(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	project, err := s.service.CreateProject(service.CreateProjectInput{AccountID: account.ID, CategoryID: catID, Name: "New Project"})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: catID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())

	projectView := NewProjectView(project, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, projectView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleUpdateProject(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var update service.UpdateProjectInput
	update.AccountID = account.ID
	update.ID = id
	// Handle form field updates - only one field per form submission
	if name := r.FormValue("name"); name != "" {
		update.Name = &name
	} else if desc := r.FormValue("description"); desc != "" {
		update.Description = &desc
	} else if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			update.Completion = &val
		}
	} else {
		// Public toggle form
		public := r.FormValue("public") == "on"
		update.Public = &public
	}

	project, err := s.service.UpdateProject(update)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: project.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())
}

func (s *App) handleGetTaskDetails(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.service.GetTaskWithWorkLogs(service.GetTaskInput{AccountID: account.ID, ID: id, Viewer: viewerFromAuth(auth)})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	taskView := NewTaskView(task, false, auth)

	if ctx.IsHTMX {
		if err := s.renderer.RenderTaskDetails(w, taskView); err != nil {
			writeServiceError(w, err)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.service.ListCategories(service.ListCategoriesInput{AccountID: account.ID, Viewer: viewerFromAuth(auth)})
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, taskView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleGetProjectDetails(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth := s.getAuthContext(w, r)
	account, auth, ok := s.resolveTenant(w, r, auth, false)
	if !ok {
		return
	}
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	project, err := s.service.GetProjectWithWorkLogs(service.GetProjectInput{AccountID: account.ID, ID: id, Viewer: viewerFromAuth(auth)})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	projectView := NewProjectView(project, false, auth)

	if ctx.IsHTMX {
		if err := s.renderer.RenderProjectDetails(w, projectView); err != nil {
			writeServiceError(w, err)
		}
		return
	}

	// Deep Linking: Render full page with details open
	cats, err := s.service.ListCategories(service.ListCategoriesInput{AccountID: account.ID, Viewer: viewerFromAuth(auth)})
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false, auth)
	}

	if err := s.renderer.RenderIndexWithDetails(w, catViews, auth, projectView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleCreateTask(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	projectID := r.PathValue("id")

	task, err := s.service.CreateTask(service.CreateTaskInput{AccountID: account.ID, ProjectID: projectID, Name: "New Task"})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: task.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())

	taskView := NewTaskView(task, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, taskView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleUpdateTask(
	w http.ResponseWriter,
	r *http.Request,
) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var update service.UpdateTaskInput
	update.AccountID = account.ID
	update.ID = id
	// Handle form field updates - only one field per form submission
	if name := r.FormValue("name"); name != "" {
		update.Name = &name
	} else if desc := r.FormValue("description"); desc != "" {
		update.Description = &desc
	} else if comp := r.FormValue("completion"); comp != "" {
		val, err := strconv.Atoi(comp)
		if err == nil {
			update.Completion = &val
		}
	} else {
		// Public toggle form
		public := r.FormValue("public") == "on"
		update.Public = &public
	}

	task, err := s.service.UpdateTask(update)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: task.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())
}

func (s *App) handleReorderCategories(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	if err := s.service.ReorderCategories(service.ReorderCategoriesInput{AccountID: account.ID, IDs: ids}); err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *App) handleReorderProjects(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	if err := s.service.ReorderProjects(service.ReorderProjectsInput{AccountID: account.ID, CategoryID: catID, ProjectIDs: ids}); err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *App) handleReorderTasks(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	projectID := r.FormValue("project_id")
	ids := r.Form["id"]

	if err := s.service.ReorderTasks(service.ReorderTasksInput{AccountID: account.ID, ProjectID: projectID, TaskIDs: ids}); err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *App) handleDeleteCategory(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	if _, err := s.service.DeleteCategory(service.DeleteCategoryInput{AccountID: account.ID, ID: id}); err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	if err := s.renderer.RenderCategoryDeleteOOB(w, id); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleDeleteProject(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	project, err := s.service.DeleteProject(service.DeleteProjectInput{AccountID: account.ID, ID: id})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: project.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	s.renderer.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())
}

func (s *App) handleDeleteTask(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	task, err := s.service.DeleteTask(service.DeleteTaskInput{AccountID: account.ID, ID: id})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: task.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	s.renderer.RenderSlideoverClear(w)
	catView := NewCategoryView(cat, true, auth)
	var buf bytes.Buffer
	if err := s.renderer.RenderCategoryOOB(&buf, catView); err != nil {
		writeServiceError(w, err)
		return
	}
	w.Write(buf.Bytes())
}

func (s *App) handleCreateProjectWorkLog(
	w http.ResponseWriter,
	r *http.Request,
) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	account, auth, ok := s.resolveTenant(w, r, auth, true)
	if !ok {
		return
	}

	ctx := parseRequestContext(r)
	projectID := r.PathValue("id")

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

	workLogInput := service.AddProjectWorkLogInput{
		AccountID:          account.ID,
		ProjectID:          projectID,
		HoursWorked:        hoursWorked,
		WorkDescription:    workDescription,
		CompletionEstimate: completionEstimate,
		CreatedAt:          customTime,
	}
	workLog, err := s.service.AddProjectWorkLog(workLogInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: workLog.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		writeServiceError(w, err)
		return
	}

	// Re-fetch project with work logs and render slideover OOB update
	project, err := s.service.GetProject(service.GetProjectInput{AccountID: account.ID, ID: projectID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	projectWorkLogs, err := s.service.ListProjectWorkLogs(service.ListProjectWorkLogsInput{AccountID: account.ID, ProjectID: projectID})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	project.WorkLogs = projectWorkLogs

	projectView := NewProjectView(project, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, projectView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleCreateTaskWorkLog(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	workLogInput := service.AddTaskWorkLogInput{
		AccountID:          account.ID,
		TaskID:             taskID,
		HoursWorked:        hoursWorked,
		WorkDescription:    workDescription,
		CompletionEstimate: completionEstimate,
		CreatedAt:          customTime,
	}
	workLog, err := s.service.AddTaskWorkLog(workLogInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	cat, err := s.service.GetCategory(service.GetCategoryInput{AccountID: account.ID, ID: workLog.CategoryID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		writeServiceError(w, err)
		return
	}

	// Re-fetch task with work logs and render slideover OOB update
	task, err := s.service.GetTask(service.GetTaskInput{AccountID: account.ID, ID: taskID, Viewer: service.OwnerViewer()})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	taskWorkLogs, err := s.service.ListTaskWorkLogs(service.ListTaskWorkLogsInput{AccountID: account.ID, TaskID: taskID})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	task.WorkLogs = taskWorkLogs

	taskView := NewTaskView(task, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, taskView); err != nil {
		writeServiceError(w, err)
	}
}
