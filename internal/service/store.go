package service

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

	GetProject(accountID string, id string) (*Project, error)
	AddProject(accountID string, catID string, name string) (*Project, error)
	UpdateProject(accountID string, project *Project) (*Project, error)
	DeleteProject(accountID string, id string) (*Project, error)
	ReorderProjects(accountID string, catID string, projectIDs []string) error

	GetTask(accountID string, id string) (*Task, error)
	AddTask(accountID string, projectID string, name string) (*Task, error)
	UpdateTask(accountID string, task *Task) (*Task, error)
	DeleteTask(accountID string, id string) (*Task, error)
	ReorderTasks(accountID string, projectID string, taskIDs []string) error

	AddWorkLogForProject(accountID string, projectID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	AddWorkLogForTask(accountID string, taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error)
	GetWorkLogsForTask(accountID string, taskID string) ([]*WorkLog, error)
	GetWorkLogsForProject(accountID string, projectID string) ([]*WorkLog, error)
	GetWorkLogsForCategory(accountID string, categoryID string) ([]*WorkLog, error)
}
