package web

import (
	"io"

	"git.sr.ht/~jakintosh/compass/internal/domain"
)

// SubtaskView is the view model for Subtask
type SubtaskView struct {
	AuthContext
	ID           string
	Name         string
	Description  string
	Completion   int
	Public       bool
	ParentPublic bool // Whether parent task (and its category) is public
	WorkLogs     []WorkLogView
	OOB          bool
	DeleteButton DeleteButtonView
}

// NewSubtaskView creates a SubtaskView from a domain Subtask
func NewSubtaskView(s *domain.Subtask, oob bool, auth AuthContext) SubtaskView {
	return SubtaskView{
		AuthContext:  auth,
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Completion:   s.Completion,
		Public:       s.Public,
		ParentPublic: s.ParentPublic,
		WorkLogs:     NewWorkLogViewsFromSubtask(s),
		OOB:          oob,
		DeleteButton: DeleteButtonView{
			URL:            "/subtasks/" + s.ID + "?csrf=" + auth.CSRFToken,
			ConfirmMessage: "Delete this subtask?",
			ButtonText:     "Delete Subtask",
		},
	}
}

// RenderSubtask renders a single subtask from its view model
func (p *Presentation) RenderSubtask(w io.Writer, view SubtaskView) error {
	return p.tmpl.ExecuteTemplate(w, "subtask.html", view)
}

// RenderSubtaskDetails renders the subtask details slideover
func (p *Presentation) RenderSubtaskDetails(w io.Writer, view SubtaskView) error {
	return p.tmpl.ExecuteTemplate(w, "subtask_details", view)
}
