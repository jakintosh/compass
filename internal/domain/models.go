package domain

type Subtask struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	Name       string `json:"name"`
	Completion int    `json:"completion"` // 0-100
}

type Task struct {
	ID          string     `json:"id"`
	CategoryID  string     `json:"category_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Completion  int        `json:"completion"` // 0-100, calculated if Subtasks > 0
	Subtasks    []*Subtask `json:"subtasks"`
}

type Category struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Tasks     []*Task `json:"tasks"`
	Collapsed bool    `json:"collapsed"`
}

// Helper methods

func (t *Task) UpdateCompletion() {
	if len(t.Subtasks) == 0 {
		return
	}
	sum := 0
	for _, s := range t.Subtasks {
		sum += s.Completion
	}
	t.Completion = sum / len(t.Subtasks)
}

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
