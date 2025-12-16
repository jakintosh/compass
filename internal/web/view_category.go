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
	Collapsed         bool
	Tasks             []TaskView
	OOB               bool
}

// NewCategoryView creates a CategoryView from a domain Category
func NewCategoryView(c *domain.Category, oob bool) CategoryView {
	view := CategoryView{
		ID:                c.ID,
		Name:              c.Name,
		Description:       c.Description,
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

// RenderCategory renders a single category from its view model
func (p *Presentation) RenderCategory(w io.Writer, view CategoryView) error {
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

// RenderCategoryDetails renders the category details slideover
func (p *Presentation) RenderCategoryDetails(w io.Writer, view CategoryView) error {
	return p.tmpl.ExecuteTemplate(w, "category_details", view)
}
