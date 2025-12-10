package domain

type Store interface {
	GetCategories() ([]*Category, error)
	GetCategory(id string) (*Category, error)
	AddCategory(name string) (*Category, error)
	UpdateCategory(cat *Category) error
	DeleteCategory(id string) error
	ReorderCategories(ids []string) error

	GetTask(id string) (*Task, error)
	AddTask(catID string, name string) (*Task, error)
	UpdateTask(task *Task) error
	DeleteTask(id string) error
	MoveTask(taskID string, newCatID string, newIndex int) error
	ReorderTasks(catID string, taskIDs []string) error

	GetSubtask(id string) (*Subtask, error)
	AddSubtask(taskID string, name string) (*Subtask, error)
	UpdateSubtask(sub *Subtask) error
	DeleteSubtask(id string) error
	ReorderSubtasks(taskID string, subIDs []string) error
}
