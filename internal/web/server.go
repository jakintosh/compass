package web

import (
	"net/http"
	"strconv"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

type Server struct {
	store        domain.Store
	router       *http.ServeMux
	presentation *Presentation
}

func NewServer(store domain.Store) (*Server, error) {
	pres, err := NewPresentation()
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:        store,
		router:       http.NewServeMux(),
		presentation: pres,
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

	// Page Routes
	s.router.HandleFunc("GET /{$}", s.handleIndex)

	// API/HTMX Routes
	s.router.HandleFunc("POST /categories", s.handleCreateCategory)
	s.router.HandleFunc("PATCH /categories/{id}", s.handleUpdateCategory)
	s.router.HandleFunc("GET /categories/{id}/details", s.handleGetCategoryDetails)
	s.router.HandleFunc("PATCH /categories/{id}/toggle-collapse", s.handleToggleCollapseCategory)
	s.router.HandleFunc("POST /categories/{id}/tasks", s.handleCreateTask)
	s.router.HandleFunc("PATCH /tasks/{id}", s.handleUpdateTask)
	s.router.HandleFunc("PATCH /tasks/{id}/toggle-expand", s.handleToggleExpandTask)
	s.router.HandleFunc("GET /tasks/{id}/details", s.handleGetTaskDetails)
	s.router.HandleFunc("POST /tasks/{id}/subtasks", s.handleCreateSubtask)
	s.router.HandleFunc("PATCH /subtasks/{id}", s.handleUpdateSubtask)
	s.router.HandleFunc("POST /categories/reorder", s.handleReorderCategories)
	s.router.HandleFunc("POST /tasks/move", s.handleMoveTask)
	s.router.HandleFunc("GET /subtasks/{id}/details", s.handleGetSubtaskDetails)
	s.router.HandleFunc("POST /subtasks/reorder", s.handleReorderSubtasks)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	catViews := make([]CategoryView, len(cats))
	for i, c := range cats {
		catViews[i] = NewCategoryView(c, false)
	}

	if err := s.presentation.RenderIndex(w, catViews); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
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

	if err := s.presentation.RenderCategory(w, NewCategoryView(cat, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleToggleCollapseCategory(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	cat, err := s.store.GetCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Handle Toggle Collapse
	cat.Collapsed = !cat.Collapsed
	s.store.UpdateCategory(cat)

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := s.presentation.RenderCategory(w, NewCategoryView(cat, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
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

	s.store.UpdateCategory(cat)

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Render OOB updates for category name (in header)
	catView := NewCategoryView(cat, true)
	if err := s.presentation.RenderCategory(w, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetCategoryDetails(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	cat, err := s.store.GetCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if ctx.IsHTMX {
		if err := s.presentation.RenderCategoryDetails(w, NewCategoryView(cat, false)); err != nil {
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
		catViews[i] = NewCategoryView(c, false)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, NewCategoryView(cat, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleToggleExpandTask(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	id := r.PathValue("id")
	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Toggle expanded state
	task.Expanded = !task.Expanded
	s.store.UpdateTask(task)

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := s.presentation.RenderTask(w, NewTaskView(task, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
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

	if err := s.presentation.RenderTask(w, NewTaskView(task, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
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

	s.store.UpdateTask(task)

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Re-fetch category for average completion
	cat, err := s.store.GetCategory(task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to view models
	taskView := NewTaskView(task, true)
	catView := NewCategoryView(cat, true)

	if err := s.presentation.RenderTaskUpdateOOB(w, taskView, catView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetSubtaskDetails(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	sub, err := s.store.GetSubtask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if ctx.IsHTMX {
		if err := s.presentation.RenderSubtaskDetails(w, NewSubtaskView(sub, false)); err != nil {
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
		catViews[i] = NewCategoryView(c, false)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, NewSubtaskView(sub, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetTaskDetails(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	id := r.PathValue("id")

	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if ctx.IsHTMX {
		if err := s.presentation.RenderTaskDetails(w, NewTaskView(task, false)); err != nil {
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
		catViews[i] = NewCategoryView(c, false)
	}

	if err := s.presentation.RenderIndexWithDetails(w, catViews, NewTaskView(task, false)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
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

	// Fetch parent task to update progress bar and show subtask inline
	task, err := s.store.GetTask(taskID)
	if err != nil {
		http.Error(w, "Failed to get parent task", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	subView := NewSubtaskView(sub, false)
	taskView := NewTaskView(task, true)

	if err := s.presentation.RenderSubtaskCreatedOOB(w, subView, taskView); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateSubtask(w http.ResponseWriter, r *http.Request) {
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

	s.store.UpdateSubtask(sub)

	if !ctx.IsHTMX {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Fetch necessary data for updates
	taskID := sub.TaskID
	if taskID == "" {
		s_check, _ := s.store.GetSubtask(sub.ID)
		if s_check != nil {
			taskID = s_check.TaskID
		}
	}

	var task *domain.Task
	var cat *domain.Category

	if taskID != "" {
		t, _ := s.store.GetTask(taskID)
		if t != nil {
			task = t
			c, _ := s.store.GetCategory(t.CategoryID)
			cat = c
		}
	}

	// Render updates
	if task != nil {
		subView := NewSubtaskView(sub, true)
		taskView := NewTaskView(task, true)
		var catView *CategoryView
		if cat != nil {
			cv := NewCategoryView(cat, true)
			catView = &cv
		}

		if err := s.presentation.RenderSubtaskUpdateOOB(w, subView, taskView, catView); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleMoveTask(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	taskID := r.FormValue("task_id")
	catID := r.FormValue("category_id")
	idxStr := r.FormValue("index")

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "Invalid index", http.StatusBadRequest)
		return
	}

	if err := s.store.MoveTask(taskID, catID, idx); err != nil {
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
