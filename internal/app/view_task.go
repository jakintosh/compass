package app

import (
	"io"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

// TaskView is the view model for Task
type TaskView struct {
	AuthContext
	ID           string
	Name         string
	Description  string
	Completion   int
	Public       bool
	ParentPublic bool // Whether parent project (and its category) is public
	TaskLogs     []TaskLogView
	OOB          bool
	DeleteButton DeleteButtonView
}

// NewTaskView creates a TaskView from a service Task.
func NewTaskView(
	s *service.Task,
	oob bool,
	auth AuthContext,
) TaskView {
	return TaskView{
		AuthContext:  auth,
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Completion:   s.Completion,
		Public:       s.Public,
		ParentPublic: s.ParentPublic,
		TaskLogs:     NewTaskLogViewsFromTask(s),
		OOB:          oob,
		DeleteButton: DeleteButtonView{
			URL:            auth.BasePath + "/tasks/" + s.ID + "?csrf=" + auth.CSRFToken,
			ConfirmMessage: "Delete this task?",
			ButtonText:     "Delete Task",
		},
	}
}

// RenderTask renders a single task from its view model
func (p *Renderer) RenderTask(
	w io.Writer,
	view TaskView,
) error {
	return p.tmpl.ExecuteTemplate(w, "task.html", view)
}

// RenderTaskDetails renders the task details slideover
func (p *Renderer) RenderTaskDetails(
	w io.Writer,
	view TaskView,
) error {
	return p.tmpl.ExecuteTemplate(w, "task_details", view)
}
