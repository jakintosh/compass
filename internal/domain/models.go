package domain

import "time"

type WorkLog struct {
	ID                 string    `json:"id"`
	CategoryID         string    `json:"category_id"`
	TaskID             string    `json:"task_id"`
	SubtaskID          string    `json:"subtask_id"` // empty string for task-level work
	HoursWorked        float64   `json:"hours_worked"`
	WorkDescription    string    `json:"work_description"`
	CompletionEstimate int       `json:"completion_estimate"` // 0-100
	CreatedAt          time.Time `json:"created_at"`
}

type Subtask struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	CategoryID  string     `json:"category_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Completion  int        `json:"completion"` // 0-100
	WorkLogs    []*WorkLog `json:"work_logs,omitempty"`
}

type Task struct {
	ID          string     `json:"id"`
	CategoryID  string     `json:"category_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Completion  int        `json:"completion"` // 0-100
	Subtasks    []*Subtask `json:"subtasks"`
	WorkLogs    []*WorkLog `json:"work_logs,omitempty"`
}

type Category struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tasks       []*Task    `json:"tasks"`
	WorkLogs    []*WorkLog `json:"work_logs,omitempty"`
}

// Helper methods

func (c *Category) AverageCompletion() int {
	if len(c.Tasks) == 0 {
		return 0
	}
	sum := 0
	for _, t := range c.Tasks {
		sum += t.Completion
	}
	return sum / len(c.Tasks)
}
