package web

import (
	"io"

	"git.sr.ht/~jakintosh/todo/internal/domain"
)

// CategoryView is the view model for Category
type CategoryView struct {
	ID                string
	Name              string
	Description       string
	AverageCompletion int
	Tasks             []TaskView
	WorkLogs          []WorkLogView
	OOB               bool
	DeleteButton      DeleteButtonView
}

// NewCategoryView creates a CategoryView from a domain Category
func NewCategoryView(c *domain.Category, oob bool) CategoryView {
	view := CategoryView{
		ID:                c.ID,
		Name:              c.Name,
		Description:       c.Description,
		AverageCompletion: c.AverageCompletion(),
		OOB:               oob,
		WorkLogs:          NewWorkLogViewsFromCategory(c),
	}
	if len(c.Tasks) > 0 {
		view.Tasks = make([]TaskView, len(c.Tasks))
		for i, t := range c.Tasks {
			view.Tasks[i] = NewTaskView(t, false)
		}
	}

	view.DeleteButton = DeleteButtonView{
		URL:            "/categories/" + c.ID,
		ConfirmMessage: "Delete this category and all its tasks?",
		ButtonText:     "Delete Category",
	}

	return view
}

// RenderCategory renders a single category from its view model
func (p *Presentation) RenderCategory(w io.Writer, view CategoryView) error {
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

// RenderCategoryDetails renders the category details slideover
func (p *Presentation) RenderCategoryDetails(w io.Writer, view CategoryView) error {
	return p.tmpl.ExecuteTemplate(w, "category_details", view)
}

// RenderCategoryOOB renders a category as an out-of-band update
func (p *Presentation) RenderCategoryOOB(w io.Writer, view CategoryView) error {
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

// RenderCategoryDeleteOOB renders OOB updates for category deletion
func (p *Presentation) RenderCategoryDeleteOOB(w io.Writer, id string) error {
	if err := p.RenderSlideoverClear(w); err != nil {
		return err
	}
	return p.tmpl.ExecuteTemplate(w, "category_delete", DeleteOOBView{ID: id})
}
