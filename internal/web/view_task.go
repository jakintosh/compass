package web

import (
	"io"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

// TaskView is the view model for Task
type TaskView struct {
	ID          string
	Name        string
	Description string
	Completion  int
	Expanded    bool
	HasSubtasks bool
	Subtasks    []SubtaskView
	OOB         bool
}

// NewTaskView creates a TaskView from a domain Task
func NewTaskView(t *domain.Task, oob bool) TaskView {
	view := TaskView{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Completion:  t.Completion,
		Expanded:    t.Expanded,
		OOB:         oob,
	}
	if len(t.Subtasks) > 0 {
		view.HasSubtasks = true
		view.Subtasks = make([]SubtaskView, len(t.Subtasks))
		for i, s := range t.Subtasks {
			view.Subtasks[i] = NewSubtaskView(s, false)
		}
	}
	return view
}

// RenderTask renders a single task from its view model
func (p *Presentation) RenderTask(w io.Writer, view TaskView) error {
	return p.tmpl.ExecuteTemplate(w, "task.html", view)
}

// RenderTaskDetails renders the task details slideover
func (p *Presentation) RenderTaskDetails(w io.Writer, view TaskView) error {
	return p.tmpl.ExecuteTemplate(w, "details", view)
}

// RenderTaskUpdateOOB renders OOB updates for a task and its parent category
func (p *Presentation) RenderTaskUpdateOOB(w io.Writer, taskView TaskView, catView CategoryView) error {
	// 1. Task percentage text
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", taskView); err != nil {
		return err
	}

	// 2. Category meta (average completion)
	if err := p.tmpl.ExecuteTemplate(w, "category_meta", catView); err != nil {
		return err
	}

	// 3. Task Name
	if err := p.tmpl.ExecuteTemplate(w, "task_name", taskView); err != nil {
		return err
	}

	// 4. Task Progress Bar Fill
	if err := p.tmpl.ExecuteTemplate(w, "task_progress_fill", taskView); err != nil {
		return err
	}

	return nil
}

// RenderTaskForUpdate renders a task with OOB enabled for updates
func (p *Presentation) RenderTaskForUpdate(w io.Writer, view TaskView) error {
	return p.tmpl.ExecuteTemplate(w, "task.html", view)
}
