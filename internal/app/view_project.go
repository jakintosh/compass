package app

import (
	"io"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

// ProjectView is the view model for Project
type ProjectView struct {
	AuthContext
	ID           string
	Name         string
	Description  string
	Completion   int
	Confidence   string
	Public       bool
	ParentPublic bool // Whether parent category is public (for disabling toggle)
	HasTasks     bool
	Tasks        []TaskView
	ProjectLogs  []ProjectLogView
	TaskLogs     []TaskLogView
	OOB          bool
	DeleteButton DeleteButtonView
}

// NewProjectView creates a ProjectView from a service Project.
func NewProjectView(
	t *service.Project,
	oob bool,
	auth AuthContext,
) ProjectView {
	view := ProjectView{
		AuthContext:  auth,
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Completion:   t.Completion,
		Confidence:   t.Confidence,
		Public:       t.Public,
		ParentPublic: t.ParentPublic,
		OOB:          oob,
	}
	if len(t.Tasks) > 0 {
		view.HasTasks = true
		view.Tasks = make([]TaskView, len(t.Tasks))
		for i, s := range t.Tasks {
			view.Tasks[i] = NewTaskView(s, false, auth)
		}
	}

	view.ProjectLogs = NewProjectLogViewsFromProject(t)
	view.TaskLogs = NewTaskLogViewsFromProject(t)

	view.DeleteButton = DeleteButtonView{
		URL:            auth.BasePath + "/projects/" + t.ID + "?csrf=" + auth.CSRFToken,
		ConfirmMessage: "Delete this project?",
		ButtonText:     "Delete Project",
	}

	return view
}

// RenderProject renders a single project from its view model
func (p *Renderer) RenderProject(
	w io.Writer,
	view ProjectView,
) error {
	return p.tmpl.ExecuteTemplate(w, "project.html", view)
}

// RenderProjectDetails renders the project details slideover
func (p *Renderer) RenderProjectDetails(
	w io.Writer,
	view ProjectView,
) error {
	return p.tmpl.ExecuteTemplate(w, "project_details", view)
}
