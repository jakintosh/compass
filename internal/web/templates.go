package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

//go:embed templates/*
var templateFS embed.FS

// Presentation handles all view-related logic and template rendering
type Presentation struct {
	tmpl *template.Template
}

// NewPresentation creates a new Presentation layer
func NewPresentation() (*Presentation, error) {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"toTaskView": func(t *domain.Task) TaskView {
			return TaskView{Task: t, Partial: false}
		},
		"toSubtaskView": func(s *domain.Subtask) SubtaskView {
			return SubtaskView{Subtask: s, Partial: false}
		},
	})

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Presentation{tmpl: tmpl}, nil
}

// View Models

type TaskView struct {
	*domain.Task
	Partial bool // Render as OOB or partial update
}

type CategoryView struct {
	*domain.Category
	Partial bool
}

type SubtaskView struct {
	*domain.Subtask
	Partial bool
}

type PageView struct {
	Categories []CategoryView
}

// Rendering Methods

func (p *Presentation) RenderIndex(w io.Writer, categories []*domain.Category) error {
	views := make([]CategoryView, len(categories))
	for i, c := range categories {
		views[i] = CategoryView{Category: c, Partial: false}
	}
	return p.tmpl.ExecuteTemplate(w, "layout.html", PageView{Categories: views})
}

func (p *Presentation) RenderCategory(w io.Writer, category *domain.Category, partial bool) error {
	view := CategoryView{
		Category: category,
		Partial:  partial,
	}
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

func (p *Presentation) RenderTask(w io.Writer, task *domain.Task, partial bool) error {
	view := TaskView{
		Task:    task,
		Partial: partial,
	}
	return p.tmpl.ExecuteTemplate(w, "task.html", view)
}

func (p *Presentation) RenderSubtask(w io.Writer, subtask *domain.Subtask, partial bool) error {
	view := SubtaskView{
		Subtask: subtask,
		Partial: partial,
	}
	return p.tmpl.ExecuteTemplate(w, "subtask.html", view) // Was previously subtask_inline
}

func (p *Presentation) RenderTaskUpdate(w io.Writer, task *domain.Task, cat *domain.Category) error {
	// 1. Task percentage text
	// <span id="task-percent-{{.ID}}" ...>%d%%</span>
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", TaskView{Task: task, Partial: true}); err != nil {
		return err
	}

	// 2. Category meta (average completion)
	// <span id="category-meta-{{.ID}}" ...>%d%% complete</span>
	if err := p.tmpl.ExecuteTemplate(w, "category_meta", CategoryView{Category: cat, Partial: true}); err != nil {
		return err
	}

	// 3. Task Name (if changed)
	// <h3 id="task-name-{{.ID}}" ...>{{.Name}}</h3>
	if err := p.tmpl.ExecuteTemplate(w, "task_name", TaskView{Task: task, Partial: true}); err != nil {
		return err
	}

	// 4. Task Progress Bar Fill (if it exists)
	// <div id="task-progress-fill-{{.ID}}" ...></div>
	// This was manually handled in handleUpdateSubtask for subtask updates, but handleUpdateTask only handled slider?
	// The slider is handled by the slider input itself or replaced?
	// In the original code `handleUpdateTask` didn't update progress bar fill, because it assumed no subtasks (slider mode).
	// But `handleUpdateSubtask` DID update parent task progress bar.

	// Let's add a method for Subtask updates which includes parent task progress bar.
	return nil
}

func (p *Presentation) RenderSubtaskUpdate(w io.Writer, subtask *domain.Subtask, task *domain.Task, cat *domain.Category) error {
	// 1. Task percentage text
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", TaskView{Task: task, Partial: true}); err != nil {
		return err
	}

	// 2. Task Progress Bar Fill
	if err := p.tmpl.ExecuteTemplate(w, "task_progress_fill", TaskView{Task: task, Partial: true}); err != nil {
		return err
	}

	// 3. Category completion
	if cat != nil {
		if err := p.tmpl.ExecuteTemplate(w, "category_meta", CategoryView{Category: cat, Partial: true}); err != nil {
			return err
		}
	}

	// 4. Subtask Name
	if err := p.tmpl.ExecuteTemplate(w, "subtask_name", SubtaskView{Subtask: subtask, Partial: true}); err != nil {
		return err
	}

	// 5. Subtask percentage
	if err := p.tmpl.ExecuteTemplate(w, "subtask_percent", SubtaskView{Subtask: subtask, Partial: true}); err != nil {
		return err
	}

	return nil
}

func (p *Presentation) RenderSubtaskCreated(w io.Writer, subtask *domain.Subtask, task *domain.Task) error {
	// 1. Render the new subtask (for the requesting list, e.g. slideover)
	if err := p.RenderSubtask(w, subtask, false); err != nil {
		return err
	}

	// 2. Re-render the Main Page Task to show the new subtask inline and update progress
	// Partial=true triggers OOB swap
	return p.RenderTask(w, task, true)
}

func (p *Presentation) RenderSubtaskDetails(w io.Writer, subtask *domain.Subtask) error {
	return p.tmpl.ExecuteTemplate(w, "subtask_details", subtask) // Details templates might not need wrapping yet?
}

func (p *Presentation) RenderTaskDetails(w io.Writer, task *domain.Task) error {
	return p.tmpl.ExecuteTemplate(w, "details", task)
}
