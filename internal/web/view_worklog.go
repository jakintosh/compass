package web

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
	TaskName           string // For category view context
	SubtaskName        string // For task/category view context
}

// NewWorkLogView creates a WorkLogView from a service WorkLog.
func NewWorkLogView(
	wl *service.WorkLog,
	taskName,
	subtaskName string,
) WorkLogView {
	return WorkLogView{
		ID:                 wl.ID,
		HoursWorked:        fmt.Sprintf("%.1f", wl.HoursWorked),
		WorkDescription:    wl.WorkDescription,
		CompletionEstimate: wl.CompletionEstimate,
		CreatedAt:          wl.CreatedAt.Format("Jan 2, 3:04 PM"),
		TaskName:           taskName,
		SubtaskName:        subtaskName,
	}
}

func NewWorkLogViewsFromSubtask(
	s *service.Task,
) []WorkLogView {
	return newWorkLogViews(s.WorkLogs, nil, nil)
}

func NewWorkLogViewsFromTask(
	t *service.Project,
) []WorkLogView {
	subtaskNames := make(map[string]string, len(t.Tasks))
	for _, s := range t.Tasks {
		subtaskNames[s.ID] = s.Name
	}
	return newWorkLogViews(t.WorkLogs, nil, subtaskNames)
}

func NewWorkLogViewsFromCategory(
	c *service.Category,
) []WorkLogView {
	taskNames := make(map[string]string, len(c.Projects))
	subtaskNames := make(map[string]string)
	for _, p := range c.Projects {
		taskNames[p.ID] = p.Name
		for _, s := range p.Tasks {
			subtaskNames[s.ID] = s.Name
		}
	}
	return newWorkLogViews(c.WorkLogs, taskNames, subtaskNames)
}

func newWorkLogViews(
	workLogs []*service.WorkLog,
	taskNames map[string]string,
	subtaskNames map[string]string,
) []WorkLogView {
	if workLogs == nil {
		return nil
	}

	views := make([]WorkLogView, len(workLogs))
	for i, wl := range workLogs {
		views[i] = NewWorkLogView(wl, taskNames[wl.ProjectID], subtaskNames[wl.TaskID])
	}
	return views
}
