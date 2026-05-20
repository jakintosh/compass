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
	ArchiveCategory(accountID string, id string) (*Category, error)
	RestoreCategory(accountID string, id string) (*Category, error)
	ReorderCategories(accountID string, ids []string) error

	GetProject(accountID string, id string) (*Project, error)
	AddProject(accountID string, catID string, name string) (*Project, error)
	UpdateProject(accountID string, project *Project) (*Project, error)
	DeleteProject(accountID string, id string) (*Project, error)
	MoveProject(accountID string, projectID string, targetCategoryID string, targetIndex int) (*Project, error)
	ArchiveProject(accountID string, id string) (*Project, error)
	RestoreProject(accountID string, id string) (*Project, error)
	ReorderProjects(accountID string, catID string, projectIDs []string) error

	GetTask(accountID string, id string) (*Task, error)
	AddTask(accountID string, projectID string, name string) (*Task, error)
	UpdateTask(accountID string, task *Task) (*Task, error)
	DeleteTask(accountID string, id string) (*Task, error)
	MoveTask(accountID string, taskID string, targetProjectID string, targetIndex int) (*Task, error)
	ArchiveTask(accountID string, id string) (*Task, error)
	RestoreTask(accountID string, id string) (*Task, error)
	ReorderTasks(accountID string, projectID string, taskIDs []string) error

	AddTaskLog(accountID string, taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*TaskLog, error)
	AddProjectLog(accountID string, projectID string, statusEstimate int, confidence string, note string, customTime *time.Time) (*ProjectLog, error)
	GetTaskLogsForTask(accountID string, taskID string) ([]*TaskLog, error)
	GetTaskLogsForProject(accountID string, projectID string) ([]*TaskLog, error)
	GetTaskLogsForCategory(accountID string, categoryID string) ([]*TaskLog, error)
	GetProjectLogsForProject(accountID string, projectID string) ([]*ProjectLog, error)
}
