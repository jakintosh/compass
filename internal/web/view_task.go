package web

import (
	"io"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

// TaskView is the view model for Task
type TaskView struct {
	ID           string
	Name         string
	Description  string
	Completion   int
	Expanded     bool
	HasSubtasks  bool
	Subtasks     []SubtaskView
	OOB          bool
	DeleteButton DeleteButtonView
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

	view.DeleteButton = DeleteButtonView{
		URL:            "/tasks/" + t.ID,
		ConfirmMessage: "Delete this task?",
		ButtonText:     "Delete Task",
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
