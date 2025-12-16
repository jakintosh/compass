package web

import (
	"bytes"
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
	tmpl := template.New("base")

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Presentation{tmpl: tmpl}, nil
}

// View Models

type TaskView struct {
	ID          string
	Name        string
	Description string
	Completion  int
	Subtasks    []SubtaskView
	OOB         bool
}

type CategoryView struct {
	ID                string
	Name              string
	AverageCompletion int
	Collapsed         bool
	Tasks             []TaskView
	OOB               bool
}

type SubtaskView struct {
	ID          string
	Name        string
	Description string
	Completion  int
	OOB         bool
}

type PageView struct {
	Categories    []CategoryView
	ActiveDetails template.HTML // Pre-rendered details for deep linking
}

// Factories

func NewSubtaskView(s *domain.Subtask, oob bool) SubtaskView {
	return SubtaskView{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Completion:  s.Completion,
		OOB:         oob,
	}
}

func NewTaskView(t *domain.Task, oob bool) TaskView {
	view := TaskView{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Completion:  t.Completion,
		OOB:         oob,
	}
	if len(t.Subtasks) > 0 {
		view.Subtasks = make([]SubtaskView, len(t.Subtasks))
		for i, s := range t.Subtasks {
			view.Subtasks[i] = NewSubtaskView(s, false)
		}
	}
	return view
}

func NewCategoryView(c *domain.Category, oob bool) CategoryView {
	view := CategoryView{
		ID:                c.ID,
		Name:              c.Name,
		AverageCompletion: c.AverageCompletion(),
		Collapsed:         c.Collapsed,
		OOB:               oob,
	}
	if len(c.Tasks) > 0 {
		view.Tasks = make([]TaskView, len(c.Tasks))
		for i, t := range c.Tasks {
			view.Tasks[i] = NewTaskView(t, false)
		}
	}
	return view
}

// Rendering Methods

func (p *Presentation) RenderIndex(w io.Writer, ctx RequestContext, categories []*domain.Category) error {
	return p.RenderIndexWithDetails(w, ctx, categories, nil)
}

func (p *Presentation) RenderIndexWithDetails(w io.Writer, ctx RequestContext, categories []*domain.Category, detailsModel interface{}) error {
	catViews := make([]CategoryView, len(categories))
	for i, c := range categories {
		catViews[i] = NewCategoryView(c, false)
	}

	pageView := PageView{Categories: catViews}

	if detailsModel != nil {
		var buf bytes.Buffer
		var tmplName string
		var viewModel any

		switch v := detailsModel.(type) {
		case *domain.Task:
			tmplName = "details"
			// Force conversion to View
			viewModel = NewTaskView(v, false)
		case *domain.Subtask:
			tmplName = "subtask_details"
			// Force conversion to View
			viewModel = NewSubtaskView(v, false)
		default:
			return fmt.Errorf("unknown details model type: %T", v)
		}

		if err := p.tmpl.ExecuteTemplate(&buf, tmplName, viewModel); err != nil {
			return err
		}
		pageView.ActiveDetails = template.HTML(buf.String())
	}

	return p.tmpl.ExecuteTemplate(w, "layout.html", pageView)
}

func (p *Presentation) RenderCategory(w io.Writer, ctx RequestContext, category *domain.Category) error {
	view := NewCategoryView(category, false)
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

func (p *Presentation) RenderTask(w io.Writer, ctx RequestContext, task *domain.Task) error {
	view := NewTaskView(task, false)
	return p.tmpl.ExecuteTemplate(w, "task.html", view)
}

func (p *Presentation) RenderSubtask(w io.Writer, ctx RequestContext, subtask *domain.Subtask) error {
	view := NewSubtaskView(subtask, false)
	return p.tmpl.ExecuteTemplate(w, "subtask.html", view)
}

func (p *Presentation) RenderTaskUpdateOOB(w io.Writer, task *domain.Task, cat *domain.Category) error {
	view := NewTaskView(task, true)

	// 1. Task percentage text
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", view); err != nil {
		return err
	}

	// 2. Category meta (average completion)
	if err := p.tmpl.ExecuteTemplate(w, "category_meta", NewCategoryView(cat, true)); err != nil {
		return err
	}

	// 3. Task Name
	if err := p.tmpl.ExecuteTemplate(w, "task_name", view); err != nil {
		return err
	}

	// 4. Task Progress Bar Fill
	if err := p.tmpl.ExecuteTemplate(w, "task_progress_fill", view); err != nil {
		return err
	}

	return nil
}

func (p *Presentation) RenderSubtaskUpdateOOB(w io.Writer, subtask *domain.Subtask, task *domain.Task, cat *domain.Category) error {
	subView := NewSubtaskView(subtask, true)
	taskView := NewTaskView(task, true)

	// 1. Parent Task percentage text
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", taskView); err != nil {
		return err
	}

	// 2. Parent Task Progress Bar Fill
	if err := p.tmpl.ExecuteTemplate(w, "task_progress_fill", taskView); err != nil {
		return err
	}

	// 3. Category completion
	if cat != nil {
		if err := p.tmpl.ExecuteTemplate(w, "category_meta", NewCategoryView(cat, true)); err != nil {
			return err
		}
	}

	// 4. Subtask Name
	if err := p.tmpl.ExecuteTemplate(w, "subtask_name", subView); err != nil {
		return err
	}

	// 5. Subtask percentage
	if err := p.tmpl.ExecuteTemplate(w, "subtask_percent", subView); err != nil {
		return err
	}

	return nil
}

func (p *Presentation) RenderSubtaskCreatedOOB(w io.Writer, subtask *domain.Subtask, task *domain.Task) error {
	// 1. Render the new subtask (standard, not OOB)
	if err := p.RenderSubtask(w, RequestContext{IsHTMX: true}, subtask); err != nil {
		return err
	}

	// 2. Update Parent Task (OOB) to show new progress etc.
	return p.RenderTaskForUpdate(w, task)
}

func (p *Presentation) RenderTaskForUpdate(w io.Writer, task *domain.Task) error {
	return p.tmpl.ExecuteTemplate(w, "task.html", NewTaskView(task, true))
}

func (p *Presentation) RenderSubtaskDetails(w io.Writer, ctx RequestContext, subtask *domain.Subtask) error {
	// With strict boundary, we convert to View here too.
	if ctx.IsHTMX {
		view := NewSubtaskView(subtask, false)
		return p.tmpl.ExecuteTemplate(w, "subtask_details", view)
	}
	return nil
}

func (p *Presentation) RenderTaskDetails(w io.Writer, ctx RequestContext, task *domain.Task) error {
	if ctx.IsHTMX {
		view := NewTaskView(task, false)
		return p.tmpl.ExecuteTemplate(w, "details", view)
	}
	return nil
}
