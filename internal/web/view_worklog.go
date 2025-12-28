package web

import (
	"fmt"

	"git.sr.ht/~jakintosh/todo/internal/domain"
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

// NewWorkLogView creates a WorkLogView from a domain WorkLog
func NewWorkLogView(wl *domain.WorkLog, taskName, subtaskName string) WorkLogView {
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

func NewWorkLogViewsFromSubtask(s *domain.Subtask) []WorkLogView {
	return newWorkLogViews(s.WorkLogs, nil, nil)
}

func NewWorkLogViewsFromTask(t *domain.Task) []WorkLogView {
	subtaskNames := make(map[string]string, len(t.Subtasks))
	for _, s := range t.Subtasks {
		subtaskNames[s.ID] = s.Name
	}
	return newWorkLogViews(t.WorkLogs, nil, subtaskNames)
}

func NewWorkLogViewsFromCategory(c *domain.Category) []WorkLogView {
	taskNames := make(map[string]string, len(c.Tasks))
	subtaskNames := make(map[string]string)
	for _, t := range c.Tasks {
		taskNames[t.ID] = t.Name
		for _, s := range t.Subtasks {
			subtaskNames[s.ID] = s.Name
		}
	}
	return newWorkLogViews(c.WorkLogs, taskNames, subtaskNames)
}

func newWorkLogViews(
	workLogs []*domain.WorkLog,
	taskNames map[string]string,
	subtaskNames map[string]string,
) []WorkLogView {
	if workLogs == nil {
		return nil
	}

	views := make([]WorkLogView, len(workLogs))
	for i, wl := range workLogs {
		views[i] = NewWorkLogView(wl, taskNames[wl.TaskID], subtaskNames[wl.SubtaskID])
	}
	return views
}
