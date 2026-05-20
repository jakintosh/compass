package app

import (
	"fmt"
	"strings"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

type TaskLogView struct {
	ID                 string
	HoursWorked        string
	WorkDescription    string
	CompletionEstimate int
	CreatedAt          string
	ProjectName        string
	TaskName           string
}

func NewTaskLogView(
	tl *service.TaskLog,
	projectName,
	taskName string,
) TaskLogView {
	return TaskLogView{
		ID:                 tl.ID,
		HoursWorked:        fmt.Sprintf("%.1f", tl.HoursWorked),
		WorkDescription:    tl.WorkDescription,
		CompletionEstimate: tl.CompletionEstimate,
		CreatedAt:          tl.CreatedAt.Format("Jan 2, 3:04 PM"),
		ProjectName:        projectName,
		TaskName:           taskName,
	}
}

type ProjectLogView struct {
	ID             string
	StatusEstimate int
	Confidence     string
	Note           string
	CreatedAt      string
}

func NewProjectLogView(
	pl *service.ProjectLog,
) ProjectLogView {
	return ProjectLogView{
		ID:             pl.ID,
		StatusEstimate: pl.StatusEstimate,
		Confidence:     strings.Title(pl.Confidence),
		Note:           pl.Note,
		CreatedAt:      pl.CreatedAt.Format("Jan 2, 3:04 PM"),
	}
}

func NewTaskLogViewsFromTask(
	t *service.Task,
) []TaskLogView {
	return newTaskLogViews(t.TaskLogs, nil, nil)
}

func NewTaskLogViewsFromProject(
	p *service.Project,
) []TaskLogView {
	taskNames := make(map[string]string, len(p.Tasks))
	for _, t := range p.Tasks {
		taskNames[t.ID] = t.Name
	}
	return newTaskLogViews(p.TaskLogs, nil, taskNames)
}

func NewTaskLogViewsFromCategory(
	c *service.Category,
) []TaskLogView {
	projectNames := make(map[string]string, len(c.Projects))
	taskNames := make(map[string]string)
	for _, p := range c.Projects {
		projectNames[p.ID] = p.Name
		for _, t := range p.Tasks {
			taskNames[t.ID] = t.Name
		}
	}
	return newTaskLogViews(c.TaskLogs, projectNames, taskNames)
}

func NewProjectLogViewsFromProject(
	p *service.Project,
) []ProjectLogView {
	if p.ProjectLogs == nil {
		return nil
	}

	views := make([]ProjectLogView, len(p.ProjectLogs))
	for i, pl := range p.ProjectLogs {
		views[i] = NewProjectLogView(pl)
	}
	return views
}

func newTaskLogViews(
	taskLogs []*service.TaskLog,
	projectNames map[string]string,
	taskNames map[string]string,
) []TaskLogView {
	if taskLogs == nil {
		return nil
	}

	views := make([]TaskLogView, len(taskLogs))
	for i, tl := range taskLogs {
		views[i] = NewTaskLogView(tl, projectNames[tl.ProjectID], taskNames[tl.TaskID])
	}
	return views
}
