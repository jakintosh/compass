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
	s.router.HandleFunc("PATCH /categories/{id}/toggle-collapse", s.handleToggleCollapseCategory)
	s.router.HandleFunc("POST /categories/{id}/tasks", s.handleCreateTask)
	s.router.HandleFunc("PATCH /tasks/{id}", s.handleUpdateTask)
	s.router.HandleFunc("GET /tasks/{id}/details", s.handleGetTaskDetails)
	s.router.HandleFunc("POST /tasks/{id}/subtasks", s.handleCreateSubtask)
	s.router.HandleFunc("PATCH /subtasks/{id}", s.handleUpdateSubtask)
	s.router.HandleFunc("POST /categories/reorder", s.handleReorderCategories)
	s.router.HandleFunc("POST /tasks/move", s.handleMoveTask)
	s.router.HandleFunc("GET /subtasks/{id}/details", s.handleGetSubtaskDetails)
	s.router.HandleFunc("POST /subtasks/reorder", s.handleReorderSubtasks)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := parseRequestContext(r)
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	if err := s.presentation.RenderIndex(w, ctx, cats); err != nil {
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

	if err := s.presentation.RenderCategory(w, ctx, cat); err != nil {
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

	if err := s.presentation.RenderCategory(w, ctx, cat); err != nil {
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

	if err := s.presentation.RenderTask(w, ctx, task); err != nil {
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

	if err := s.presentation.RenderTaskUpdateOOB(w, task, cat); err != nil {
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
		if err := s.presentation.RenderSubtaskDetails(w, ctx, sub); err != nil {
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
	if err := s.presentation.RenderIndexWithDetails(w, ctx, cats, sub); err != nil {
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

	if !ctx.IsHTMX {
		if err := s.presentation.RenderTaskDetails(w, ctx, task); err != nil {
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
	if err := s.presentation.RenderIndexWithDetails(w, ctx, cats, task); err != nil {
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

	if err := s.presentation.RenderSubtaskCreatedOOB(w, sub, task); err != nil {
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
		if err := s.presentation.RenderSubtaskUpdateOOB(w, sub, task, cat); err != nil {
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
