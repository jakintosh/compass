package domain

import "time"

type Store interface {
	GetCategories() ([]*Category, error)
	GetCategory(id string) (*Category, error)
	AddCategory(name string) (*Category, error)
	UpdateCategory(cat *Category) (*Category, error)
	DeleteCategory(id string) (*Category, error)
	ReorderCategories(ids []string) error

	GetTask(id string) (*Task, error)
	AddTask(catID string, name string) (*Task, error)
	UpdateTask(task *Task) (*Task, error)
	DeleteTask(id string) (*Task, error)
	ReorderTasks(catID string, taskIDs []string) error

	GetSubtask(id string) (*Subtask, error)
	AddSubtask(taskID string, name string) (*Subtask, error)
	UpdateSubtask(sub *Subtask) (*Subtask, error)
	DeleteSubtask(id string) (*Subtask, error)
	ReorderSubtasks(taskID string, subIDs []string) error

	AddWorkLogForTask(taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	AddWorkLogForSubtask(subtaskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	GetWorkLogsForSubtask(subtaskID string) ([]*WorkLog, error)
	GetWorkLogsForTask(taskID string) ([]*WorkLog, error)
	GetWorkLogsForCategory(categoryID string) ([]*WorkLog, error)
}
