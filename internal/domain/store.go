package domain

import "time"

type Store interface {
	GetAccountByHandle(handle string) (*Account, error)
	GetAccountBySubject(subject string) (*Account, error)
	UpsertAccount(subject string, handle string, refreshedAt time.Time) (*Account, error)

	GetCategories(accountID string) ([]*Category, error)
	GetCategory(accountID string, id string) (*Category, error)
	AddCategory(accountID string, name string) (*Category, error)
	UpdateCategory(accountID string, cat *Category) (*Category, error)
	DeleteCategory(accountID string, id string) (*Category, error)
	ReorderCategories(accountID string, ids []string) error

	GetTask(accountID string, id string) (*Task, error)
	AddTask(accountID string, catID string, name string) (*Task, error)
	UpdateTask(accountID string, task *Task) (*Task, error)
	DeleteTask(accountID string, id string) (*Task, error)
	ReorderTasks(accountID string, catID string, taskIDs []string) error

	GetSubtask(accountID string, id string) (*Subtask, error)
	AddSubtask(accountID string, taskID string, name string) (*Subtask, error)
	UpdateSubtask(accountID string, sub *Subtask) (*Subtask, error)
	DeleteSubtask(accountID string, id string) (*Subtask, error)
	ReorderSubtasks(accountID string, taskID string, subIDs []string) error

	AddWorkLogForTask(accountID string, taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	AddWorkLogForSubtask(accountID string, subtaskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	GetWorkLogsForSubtask(accountID string, subtaskID string) ([]*WorkLog, error)
	GetWorkLogsForTask(accountID string, taskID string) ([]*WorkLog, error)
	GetWorkLogsForCategory(accountID string, categoryID string) ([]*WorkLog, error)
}
