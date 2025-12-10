package web

import (
	"fmt"
	"net/http"
	"strconv"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

type Server struct {
	store  domain.Store
	router *http.ServeMux
}

func NewServer(store domain.Store) *Server {
	s := &Server{
		store:  store,
		router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Static Files
	fs := http.FileServer(http.Dir("internal/web/static"))
	s.router.Handle("/static/", http.StripPrefix("/static/", fs))

	// Page Routes
	s.router.HandleFunc("/", s.handleIndex)

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
	cats, err := s.store.GetCategories()
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]interface{}{
		"Categories": cats,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	cat, err := s.store.AddCategory("New Category")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "category", cat); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleToggleCollapseCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id") // Go 1.22+ routing
	cat, err := s.store.GetCategory(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	cat.Collapsed = !cat.Collapsed
	s.store.UpdateCategory(cat)

	// HTMX will swap the button itself or the container?
	// The frontend requests: hx-target="#category-{{.ID}} .tasks-container" hx-swap="none"
	// But actually we want to re-render the button to rotate the arrow AND toggle the container visibility.
	// The current frontend implementation uses `hx-swap="none"` for the button? That won't update the UI.
	// Let's re-render the whole category to be safe and simple, or improved logic.
	// Ideally we just toggle the class on the client, but for persistence we notify server.
	// Let's re-render the category header or simple return nothing if we use client-side toggle?
	// But the user wants server-side rendering logic.

	// Let's actually fix the template to swap the whole category or specific parts.
	// For now, let's just return 200 OK and let `hx-swap="none"` do nothing,
	// BUT wait, the UI won't update!

	// Better approach:
	// Return the updated Category template.
	// Frontend `hx-target="#category-{{.ID}}"` `hx-swap="outerHTML"`
	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "category", cat); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	catID := r.PathValue("id")
	task, err := s.store.AddTask(catID, "New Task")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "task", task); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
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

	// Re-fetch category for average completion
	cat, err := s.store.GetCategory(task.CategoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// rendering multiple unrelated parts using OOB swaps requires them to be top level elements in string
	// or we can execute multiple templates and concat.
	// Let's execute task completion part and category header part.

	// But first, we need templates for these small bits.
	// Or we can just render the specific elements with `hx-swap-oob="true"`

	// 1. Task percentage text
	// <span id="task-percent-{{.ID}}" class="task-percent" hx-swap-oob="true">{{.Completion}}%</span>
	fmt.Fprintf(w, `<span id="task-percent-%s" class="task-percent" hx-swap-oob="true">%d%%</span>`, task.ID, task.Completion)

	// 2. Category meta (average completion)
	// <span id="category-meta-{{.ID}}" class="category-meta" hx-swap-oob="true">{{.AverageCompletion}}% complete</span>
	fmt.Fprintf(w, `<span id="category-meta-%s" class="category-meta" hx-swap-oob="true">%d%% complete</span>`, cat.ID, cat.AverageCompletion())

	// 3. If we want to be fancy, updating the task progress bar if it has one (if it has subtasks)
	// But `handleUpdateTask` mainly handles the slider which only appears if NO subtasks.
	// So we don't need to update progress bar here, unless the user edited completion manually via other means?
	// The slider is the main way.

	// 3. Task Name (if changed)
	// <h3 id="task-name-{{.ID}}" class="task-name" hx-swap-oob="true">{{.Name}}</h3>
	fmt.Fprintf(w, `<h3 id="task-name-%s" class="task-name" hx-swap-oob="true">%s</h3>`, task.ID, task.Name)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetSubtaskDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sub, err := s.store.GetSubtask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "subtask_details", sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetTaskDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "details", task); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	sub, err := s.store.AddSubtask(taskID, "New Subtask")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 1. Render the new subtask for the details slideover list
	tmpl := getTemplates()
	if err := tmpl.ExecuteTemplate(w, "subtask", sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Re-render the Main Page Task to show the new subtask inline and update progress
	// We need to fetch the task with all subtasks loaded
	task, err := s.store.GetTask(taskID)
	if err == nil {
		// hx-swap-oob="outerHTML:#task-{ID}"
		// Since we can't easily add attributes to the template output without modifying the template or wrapping it,
		// and the `task` template outputs the wrapper `div id="task-{ID}"`,
		// we can output it and HTMX OOB logic will pick it up if we tag it?
		// Actually, standard OOB requires `hx-swap-oob="true"` on the element.
		// Our `task` template doesn't have that attribute.
		// We can wrap it in a `<div hx-swap-oob="outerHTML:#task-{ID}">...</div>`?
		// No, `outerHTML` swap replaces the target with the content.
		// If we use `hx-swap-oob="true"`, it replaces the element with matching ID.
		// Our `task` template starts with `<div class="task-wrapper" id="task-{{.ID}}" ...>`.
		// If we create a temporary template or modify the data to include a flag?
		// Or easier: Just print the OOB wrapper around the rendered template?

		// If we wrap it: `<div id="task-{ID}" hx-swap-oob="outerHTML"> (rendered task) </div>`
		// The inner rendered task has `id="task-{ID}"`.
		// This might cause nesting issues or ID conflicts during swap if not careful.
		// Correct way: The top level element in the OOB response should have `hx-swap-oob="true"` (or "outerHTML:...") AND the ID of the target.
		// Since our template output ALREADY has the ID, we just need to inject `hx-swap-oob="true"`.

		// Hacky but effective: Render to string, inject attribute?
		// Or: Wrapper approach:
		// <template hx-swap-oob="outerHTML:#task-{ID}"> ... content ... </template>

		// Let's use the `<div hx-swap-oob>` wrapper replacing the target ID.
		// `<div id="task-{ID}" hx-swap-oob="true"> ... </div>` won't work because the template output is ALSO that div.
		// We can wrap in a `<template>` tag?
		// `fmt.Fprintf(w, `<div hx-swap-oob="outerHTML:#task-%s">`, task.ID)`
		// Then execute template.
		// Then `fmt.Fprint(w, "</div>")`

		fmt.Fprintf(w, `<div hx-swap-oob="outerHTML:#task-%s">`, task.ID)
		tmpl.ExecuteTemplate(w, "task", task)
		fmt.Fprint(w, `</div>`)
	}
}

func (s *Server) handleUpdateSubtask(w http.ResponseWriter, r *http.Request) {
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

	// We need to update:
	// 1. Subtask completion text/slider? Slider updates itself visually, but text might exist?
	// 2. Parent Task progress bar and completion text.
	// 3. Category average completion.

	taskID := sub.TaskID
	// If missing (shouldn't be), fetch it
	if taskID == "" {
		// This happens if store implementation of UpdateSubtask didn't populate it and we didn't have it.
		// Our SQLite store helper sets it? No, UpdateSubtask in store just blindly updates.
		// Let's safe fetch.
		s, _ := s.store.GetSubtask(sub.ID) // re-fetch to get task_id
		if s != nil {
			taskID = s.TaskID
		}
	}

	if taskID != "" {
		task, _ := s.store.GetTask(taskID)
		if task != nil {
			cat, _ := s.store.GetCategory(task.CategoryID)

			// OOB Swaps

			// 1. Task completion text (if visible? it is visible for tasks with subtasks too)
			fmt.Fprintf(w, `<span id="task-percent-%s" class="task-percent" hx-swap-oob="true">%d%%</span>`, task.ID, task.Completion)

			// 2. Task Progress Bar Fill
			// <div id="task-progress-fill-{{.ID}}" class="task-progress-fill" style="width: {{.Completion}}%" hx-swap-oob="true"></div>
			fmt.Fprintf(w, `<div id="task-progress-fill-%s" class="task-progress-fill" style="width: %d%%" hx-swap-oob="true"></div>`, task.ID, task.Completion)

			// 3. Category completion
			if cat != nil {
				fmt.Fprintf(w, `<span id="category-meta-%s" class="category-meta" hx-swap-oob="true">%d%% complete</span>`, cat.ID, cat.AverageCompletion())
			}

			// 4. Subtask Name (if changed)
			// <span id="subtask-name-{{.ID}}" class="task-name" hx-swap-oob="true">{{.Name}}</span>
			// Need to be careful with swapping "task-name" span if it's inside something else.
			// In template: <span id="subtask-name-{{.ID}}" class="task-name">{{.Name}}</span>
			fmt.Fprintf(w, `<span id="subtask-name-%s" class="task-name" hx-swap-oob="true">%s</span>`, sub.ID, sub.Name)

			// 5. Subtask percentage text (if we add slider on main page)
			// <span id="subtask-percent-{{.ID}}" ...>%d%%</span>
			fmt.Fprintf(w, `<span id="subtask-percent-%s" class="task-percent" hx-swap-oob="true">%d%%</span>`, sub.ID, sub.Completion)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMoveTask(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderSubtasks(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusOK)
}
