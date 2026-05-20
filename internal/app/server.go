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
	mux.HandleFunc("POST /{id}/logs", s.handleCreateProjectLog)

	return mux
}

func (s *App) buildTaskRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /reorder", s.handleReorderTasks)
	mux.HandleFunc("PATCH /{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /{id}", s.handleDeleteTask)
	mux.HandleFunc("GET /{id}/details", s.handleGetTaskDetails)
	mux.HandleFunc("POST /{id}/logs", s.handleCreateTaskLog)

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
	ctx, err := s.resolveAuthContext(w, r)
	if err != nil {
		log.Printf("failed to resolve authenticated account: %v", err)
	}
	return ctx
}

func (s *App) resolveAuthContext(
	w http.ResponseWriter,
	r *http.Request,
) (
	AuthContext,
	error,
) {
	ctx := AuthContext{
		IsAuthenticated: false,
		LoginURL:        s.loginURL(r),
		LogoutURL:       s.auth.LogoutURL,
	}

	accessToken, csrfToken, err := s.auth.Verifier.VerifyAuthorizationGetCSRF(w, r)
	if err != nil {
		return ctx, nil
	}

	ctx.IsAuthenticated = true
	ctx.Subject = accessToken.Subject()
	ctx.CSRFToken = csrfToken
	ctx.LoginURL = s.loginURL(r)
	account, err := s.accountForToken(accessToken)
	if err == nil && account != nil {
		ctx.AccountID = account.ID
		ctx.Handle = account.Handle
		return ctx, nil
	}
	return ctx, err
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
		s.renderAccountSetupFailure(w, r, auth, err)
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
	auth, setupErr := s.resolveAuthContext(w, r)
	if setupErr != nil {
		log.Printf("failed to resolve authenticated account: %v", setupErr)
		s.renderAccountSetupFailure(w, r, auth, setupErr)
		return
	}
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

	input := service.ListCategoriesInput{
		AccountID: account.ID,
		Viewer:    viewerFromAuth(auth),
	}
	cats, err := s.service.ListCategories(input)
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
	input := service.GetAccountBySubjectInput{
		Subject: subject,
	}
	account, err := s.service.GetAccountBySubject(input)
	if err == nil && !s.shouldRefreshProfile(account) {
		return account, nil
	}
	if err != nil && !errors.Is(err, service.ErrNotFound) {
		return nil, transientAccountSetupError(
			"Compass could not read your account setup. This is probably temporary; try signing in again in a few minutes.",
			err,
		)
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
			return nil, transientAccountSetupError(
				"Compass could not reach Consent to finish setting up your account. Try signing in again in a few minutes.",
				fetchErr,
			)
		}
		if userInfo == nil {
			return nil, userActionAccountSetupError(
				"Consent did not return profile information for this account. Try signing in again and make sure Compass can access your profile.",
				errors.New("consent profile response missing user info"),
			)
		}
		if userInfo.Sub != subject {
			return nil, userActionAccountSetupError(
				"Consent returned profile information for a different account. Sign in again with the Consent account you want to use for Compass.",
				errors.New("consent profile subject mismatch"),
			)
		}
		if userInfo.Profile == nil {
			return nil, userActionAccountSetupError(
				"Compass needs your Consent profile handle to create your account. Try signing in again and grant profile access.",
				errors.New("consent profile missing handle"),
			)
		}
		handle = strings.TrimSpace(userInfo.Profile.Handle)
		if handle == "" {
			return nil, userActionAccountSetupError(
				"Your Consent profile does not have a usable handle yet. Add or update your handle in Consent, then sign in again.",
				errors.New("consent profile missing handle"),
			)
		}
	}

	upsertInput := service.UpsertAccountInput{
		Subject:     subject,
		Handle:      handle,
		RefreshedAt: refreshedAt,
	}
	account, err = s.service.UpsertAccount(upsertInput)
	if err != nil {
		if errors.Is(err, service.ErrAccountHandleConflict) {
			return nil, userActionAccountSetupError(
				"That Consent handle is already attached to another Compass account. Update your Consent handle or sign in with the account that owns it, then try again.",
				err,
			)
		}
		return nil, transientAccountSetupError(
			"Compass could not save your account setup. This is probably temporary; try signing in again in a few minutes.",
			err,
		)
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
	input := service.GetAccountByHandleInput{
		Handle: handle,
	}
	account, err := s.service.GetAccountByHandle(input)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			s.renderUserNotFound(w, auth, handle)
			return nil, auth, false
		}
		writeServiceError(w, err)
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
	input := service.CreateCategoryInput{
		AccountID: account.ID,
		Name:      "New Category",
	}
	cat, err := s.service.CreateCategory(input)
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

	input := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        id,
		Viewer:    viewerFromAuth(auth),
	}
	cat, err := s.service.GetCategoryWithTaskLogs(input)
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
	listInput := service.ListCategoriesInput{
		AccountID: account.ID,
		Viewer:    viewerFromAuth(auth),
	}
	cats, err := s.service.ListCategories(listInput)
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

	input := service.CreateProjectInput{
		AccountID:  account.ID,
		CategoryID: catID,
		Name:       "New Project",
	}
	project, err := s.service.CreateProject(input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render it as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        catID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        project.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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

	input := service.GetTaskInput{
		AccountID: account.ID,
		ID:        id,
		Viewer:    viewerFromAuth(auth),
	}
	task, err := s.service.GetTaskWithLogs(input)
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
	listInput := service.ListCategoriesInput{
		AccountID: account.ID,
		Viewer:    viewerFromAuth(auth),
	}
	cats, err := s.service.ListCategories(listInput)
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

	input := service.GetProjectInput{
		AccountID: account.ID,
		ID:        id,
		Viewer:    viewerFromAuth(auth),
	}
	project, err := s.service.GetProjectWithLogs(input)
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
	listInput := service.ListCategoriesInput{
		AccountID: account.ID,
		Viewer:    viewerFromAuth(auth),
	}
	cats, err := s.service.ListCategories(listInput)
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

	input := service.CreateTaskInput{
		AccountID: account.ID,
		ProjectID: projectID,
		Name:      "New Task",
	}
	task, err := s.service.CreateTask(input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Fetch parent category and render it as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        task.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        task.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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

	input := service.ReorderCategoriesInput{
		AccountID: account.ID,
		IDs:       ids,
	}
	if err := s.service.ReorderCategories(input); err != nil {
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

	input := service.ReorderProjectsInput{
		AccountID:  account.ID,
		CategoryID: catID,
		ProjectIDs: ids,
	}
	if err := s.service.ReorderProjects(input); err != nil {
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

	input := service.ReorderTasksInput{
		AccountID: account.ID,
		ProjectID: projectID,
		TaskIDs:   ids,
	}
	if err := s.service.ReorderTasks(input); err != nil {
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

	input := service.DeleteCategoryInput{
		AccountID: account.ID,
		ID:        id,
	}
	if _, err := s.service.DeleteCategory(input); err != nil {
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

	input := service.DeleteProjectInput{
		AccountID: account.ID,
		ID:        id,
	}
	project, err := s.service.DeleteProject(input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        project.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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

	input := service.DeleteTaskInput{
		AccountID: account.ID,
		ID:        id,
	}
	task, err := s.service.DeleteTask(input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category after deletion and render it as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        task.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
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

func (s *App) handleCreateProjectLog(
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

	statusEstimate, err := strconv.Atoi(r.FormValue("status_estimate"))
	if err != nil {
		http.Error(w, "Invalid status_estimate value", http.StatusBadRequest)
		return
	}

	confidence := r.FormValue("confidence")
	note := r.FormValue("note")

	// Parse optional custom timestamp
	var customTime *time.Time
	if r.FormValue("use_custom_time") == "on" {
		if ct := r.FormValue("custom_time"); ct != "" {
			if parsed, err := time.ParseInLocation("2006-01-02T15:04", ct, time.Local); err == nil {
				customTime = &parsed
			}
		}
	}

	projectLogInput := service.AddProjectLogInput{
		AccountID:      account.ID,
		ProjectID:      projectID,
		StatusEstimate: statusEstimate,
		Confidence:     confidence,
		Note:           note,
		CreatedAt:      customTime,
	}
	projectLog, err := s.service.AddProjectLog(projectLogInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        projectLog.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		writeServiceError(w, err)
		return
	}

	// Re-fetch project logs and task activity, then render the slideover OOB update.
	getProjectInput := service.GetProjectInput{
		AccountID: account.ID,
		ID:        projectID,
		Viewer:    service.OwnerViewer(),
	}
	project, err := s.service.GetProject(getProjectInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	listProjectLogsInput := service.ListProjectLogsInput{
		AccountID: account.ID,
		ProjectID: projectID,
	}
	projectLogs, err := s.service.ListProjectLogs(listProjectLogsInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	project.ProjectLogs = projectLogs
	listTaskLogsInput := service.ListProjectTaskLogsInput{
		AccountID: account.ID,
		ProjectID: projectID,
	}
	taskLogs, err := s.service.ListProjectTaskLogs(listTaskLogsInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	project.TaskLogs = taskLogs

	projectView := NewProjectView(project, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, projectView); err != nil {
		writeServiceError(w, err)
	}
}

func (s *App) handleCreateTaskLog(
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

	taskLogInput := service.AddTaskLogInput{
		AccountID:          account.ID,
		TaskID:             taskID,
		HoursWorked:        hoursWorked,
		WorkDescription:    workDescription,
		CompletionEstimate: completionEstimate,
		CreatedAt:          customTime,
	}
	taskLog, err := s.service.AddTaskLog(taskLogInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if !ctx.IsHTMX {
		http.Redirect(w, r, auth.BasePath+"/", http.StatusSeeOther)
		return
	}

	// Re-fetch category and render as OOB
	getCategoryInput := service.GetCategoryInput{
		AccountID: account.ID,
		ID:        taskLog.CategoryID,
		Viewer:    service.OwnerViewer(),
	}
	cat, err := s.service.GetCategory(getCategoryInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	catView := NewCategoryView(cat, true, auth)
	if err := s.renderer.RenderCategoryOOB(w, catView); err != nil {
		writeServiceError(w, err)
		return
	}

	// Re-fetch task logs and render the slideover OOB update.
	getTaskInput := service.GetTaskInput{
		AccountID: account.ID,
		ID:        taskID,
		Viewer:    service.OwnerViewer(),
	}
	task, err := s.service.GetTask(getTaskInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	listTaskLogsInput := service.ListTaskLogsInput{
		AccountID: account.ID,
		TaskID:    taskID,
	}
	taskLogs, err := s.service.ListTaskLogs(listTaskLogsInput)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	task.TaskLogs = taskLogs

	taskView := NewTaskView(task, false, auth)
	if err := s.renderer.RenderSlideoverWithDetails(w, taskView); err != nil {
		writeServiceError(w, err)
	}
}
