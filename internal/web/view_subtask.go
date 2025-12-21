package web

import (
	"io"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

// SubtaskView is the view model for Subtask
type SubtaskView struct {
	ID           string
	Name         string
	Description  string
	Completion   int
	OOB          bool
	DeleteButton DeleteButtonView
}

// NewSubtaskView creates a SubtaskView from a domain Subtask
func NewSubtaskView(s *domain.Subtask, oob bool) SubtaskView {
	return SubtaskView{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Completion:  s.Completion,
		OOB:         oob,
		DeleteButton: DeleteButtonView{
			URL:            "/subtasks/" + s.ID,
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

// RenderSubtaskUpdateOOB renders OOB updates for a subtask and its parent task/category
func (p *Presentation) RenderSubtaskUpdateOOB(w io.Writer, subView SubtaskView, taskView TaskView, catView *CategoryView) error {
	// 1. Parent Task percentage text
	if err := p.tmpl.ExecuteTemplate(w, "task_percent", taskView); err != nil {
		return err
	}

	// 2. Parent Task Progress Bar Fill
	if err := p.tmpl.ExecuteTemplate(w, "task_progress_fill", taskView); err != nil {
		return err
	}

	// 3. Category completion (optional)
	if catView != nil {
		if err := p.tmpl.ExecuteTemplate(w, "category_meta", *catView); err != nil {
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

// RenderSubtaskCreatedOOB renders a new subtask and OOB updates for the parent task
func (p *Presentation) RenderSubtaskCreatedOOB(w io.Writer, subView SubtaskView, taskView TaskView) error {
	// 1. Render the new subtask (standard, not OOB)
	if err := p.RenderSubtask(w, subView); err != nil {
		return err
	}

	// 2. Update Parent Task (OOB) to show new progress etc.
	return p.RenderTaskForUpdate(w, taskView)
}

// RenderSubtaskDeleteOOB renders OOB updates for subtask deletion
func (p *Presentation) RenderSubtaskDeleteOOB(w io.Writer, id string) error {
	if err := p.RenderSlideoverClear(w); err != nil {
		return err
	}
	return p.tmpl.ExecuteTemplate(w, "subtask_delete", DeleteOOBView{ID: id})
}
