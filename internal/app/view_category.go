package app

import (
	"io"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

// CategoryView is the view model for Category
type CategoryView struct {
	AuthContext
	ID                string
	Name              string
	Description       string
	Public            bool
	AverageCompletion int
	Projects          []ProjectView
	TaskLogs          []TaskLogView
	OOB               bool
	DeleteButton      DeleteButtonView
}

// NewCategoryView creates a CategoryView from a service Category
func NewCategoryView(
	c *service.Category,
	oob bool,
	auth AuthContext,
) CategoryView {
	view := CategoryView{
		AuthContext:       auth,
		ID:                c.ID,
		Name:              c.Name,
		Description:       c.Description,
		Public:            c.Public,
		AverageCompletion: c.AverageCompletion(),
		OOB:               oob,
		TaskLogs:          NewTaskLogViewsFromCategory(c),
	}
	if len(c.Projects) > 0 {
		view.Projects = make([]ProjectView, len(c.Projects))
		for i, p := range c.Projects {
			view.Projects[i] = NewProjectView(p, false, auth)
		}
	}

	view.DeleteButton = DeleteButtonView{
		URL:            auth.BasePath + "/categories/" + c.ID + "?csrf=" + auth.CSRFToken,
		ConfirmMessage: "Delete this category and all its projects?",
		ButtonText:     "Delete Category",
	}

	return view
}

// RenderCategory renders a single category from its view model
func (p *Renderer) RenderCategory(
	w io.Writer,
	view CategoryView,
) error {
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

// RenderCategoryDetails renders the category details slideover
func (p *Renderer) RenderCategoryDetails(
	w io.Writer,
	view CategoryView,
) error {
	return p.tmpl.ExecuteTemplate(w, "category_details", view)
}

// RenderCategoryOOB renders a category as an out-of-band update
func (p *Renderer) RenderCategoryOOB(
	w io.Writer,
	view CategoryView,
) error {
	return p.tmpl.ExecuteTemplate(w, "category.html", view)
}

// RenderCategoryDeleteOOB renders OOB updates for category deletion
func (p *Renderer) RenderCategoryDeleteOOB(
	w io.Writer,
	id string,
) error {
	if err := p.RenderSlideoverClear(w); err != nil {
		return err
	}
	return p.tmpl.ExecuteTemplate(w, "category_delete", DeleteOOBView{ID: id})
}
