package web

import (
	"io"

	"git.sr.ht/~jakintosh/compass/internal/domain"
)

// TaskView is the view model for Task
type TaskView struct {
	AuthContext
	ID           string
	Name         string
	Description  string
	Completion   int
	Public       bool
	ParentPublic bool // Whether parent category is public (for disabling toggle)
	HasSubtasks  bool
	Subtasks     []SubtaskView
	WorkLogs     []WorkLogView
	OOB          bool
	DeleteButton DeleteButtonView
}

// NewTaskView creates a TaskView from a domain Task
func NewTaskView(t *domain.Task, oob bool, auth AuthContext) TaskView {
	view := TaskView{
		AuthContext:  auth,
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Completion:   t.Completion,
		Public:       t.Public,
		ParentPublic: t.ParentPublic,
		OOB:          oob,
	}
	if len(t.Subtasks) > 0 {
		view.HasSubtasks = true
		view.Subtasks = make([]SubtaskView, len(t.Subtasks))
		for i, s := range t.Subtasks {
			view.Subtasks[i] = NewSubtaskView(s, false, auth)
		}
	}

	view.WorkLogs = NewWorkLogViewsFromTask(t)

	view.DeleteButton = DeleteButtonView{
		URL:            "/tasks/" + t.ID + "?csrf=" + auth.CSRFToken,
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
