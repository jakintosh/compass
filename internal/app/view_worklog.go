package app

import (
	"fmt"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

// WorkLogView is the view model for WorkLog
type WorkLogView struct {
	ID                 string
	HoursWorked        string // Formatted as string for display
	WorkDescription    string
	CompletionEstimate int
	CreatedAt          string // Formatted timestamp
	ProjectName        string // For category view context
	TaskName           string // For project/category view context
}

// NewWorkLogView creates a WorkLogView from a service WorkLog.
func NewWorkLogView(
	wl *service.WorkLog,
	projectName,
	taskName string,
) WorkLogView {
	return WorkLogView{
		ID:                 wl.ID,
		HoursWorked:        fmt.Sprintf("%.1f", wl.HoursWorked),
		WorkDescription:    wl.WorkDescription,
		CompletionEstimate: wl.CompletionEstimate,
		CreatedAt:          wl.CreatedAt.Format("Jan 2, 3:04 PM"),
		ProjectName:        projectName,
		TaskName:           taskName,
	}
}

func NewWorkLogViewsFromTask(
	s *service.Task,
) []WorkLogView {
	return newWorkLogViews(s.WorkLogs, nil, nil)
}

func NewWorkLogViewsFromProject(
	t *service.Project,
) []WorkLogView {
	taskNames := make(map[string]string, len(t.Tasks))
	for _, s := range t.Tasks {
		taskNames[s.ID] = s.Name
	}
	return newWorkLogViews(t.WorkLogs, nil, taskNames)
}

func NewWorkLogViewsFromCategory(
	c *service.Category,
) []WorkLogView {
	projectNames := make(map[string]string, len(c.Projects))
	taskNames := make(map[string]string)
	for _, p := range c.Projects {
		projectNames[p.ID] = p.Name
		for _, s := range p.Tasks {
			taskNames[s.ID] = s.Name
		}
	}
	return newWorkLogViews(c.WorkLogs, projectNames, taskNames)
}

func newWorkLogViews(
	workLogs []*service.WorkLog,
	projectNames map[string]string,
	taskNames map[string]string,
) []WorkLogView {
	if workLogs == nil {
		return nil
	}

	views := make([]WorkLogView, len(workLogs))
	for i, wl := range workLogs {
		views[i] = NewWorkLogView(wl, projectNames[wl.ProjectID], taskNames[wl.TaskID])
	}
	return views
}
