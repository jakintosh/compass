package service

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	categories []*Category
	category   *Category
	project    *Project
	task       *Task
	workLog    *WorkLog
	err        error

	lastAccountID        string
	lastName             string
	lastCategoryID       string
	lastProjectID        string
	lastTaskID           string
	lastTargetCategoryID string
	lastTargetProjectID  string
	lastTargetIndex      int
	lastProjectIDs       []string
	lastTaskIDs          []string
}

func (f *fakeStore) GetAccountByHandle(handle string) (*Account, error) {
	return nil, f.err
}

func (f *fakeStore) GetAccountBySubject(subject string) (*Account, error) {
	return nil, f.err
}

func (f *fakeStore) UpsertAccount(subject string, handle string, refreshedAt time.Time) (*Account, error) {
	return &Account{ConsentSubject: subject, Handle: handle, ProfileRefreshedAt: refreshedAt}, f.err
}

func (f *fakeStore) GetCategories(accountID string) ([]*Category, error) {
	f.lastAccountID = accountID
	return f.categories, f.err
}

func (f *fakeStore) GetCategory(accountID string, id string) (*Category, error) {
	f.lastAccountID = accountID
	f.lastCategoryID = id
	if f.category != nil {
		return f.category, f.err
	}
	return &Category{ID: id, Status: "active", Public: true}, f.err
}

func (f *fakeStore) AddCategory(accountID string, name string) (*Category, error) {
	f.lastAccountID = accountID
	f.lastName = name
	return &Category{ID: "category-1", Name: name, Status: "active", Public: true}, f.err
}

func (f *fakeStore) UpdateCategory(accountID string, cat *Category) (*Category, error) {
	f.lastAccountID = accountID
	f.category = cat
	return cat, f.err
}

func (f *fakeStore) DeleteCategory(accountID string, id string) (*Category, error) {
	f.lastAccountID = accountID
	f.lastCategoryID = id
	return &Category{ID: id}, f.err
}

func (f *fakeStore) ArchiveCategory(accountID string, id string) (*Category, error) {
	f.lastAccountID = accountID
	f.lastCategoryID = id
	return &Category{ID: id, Status: "archived"}, f.err
}

func (f *fakeStore) RestoreCategory(accountID string, id string) (*Category, error) {
	f.lastAccountID = accountID
	f.lastCategoryID = id
	return &Category{ID: id, Status: "active"}, f.err
}

func (f *fakeStore) ReorderCategories(accountID string, ids []string) error {
	f.lastAccountID = accountID
	return f.err
}

func (f *fakeStore) GetProject(accountID string, id string) (*Project, error) {
	f.lastAccountID = accountID
	f.lastProjectID = id
	if f.project != nil {
		return f.project, f.err
	}
	return &Project{ID: id, Status: "active", Public: true, ParentPublic: true}, f.err
}

func (f *fakeStore) AddProject(accountID string, catID string, name string) (*Project, error) {
	f.lastAccountID = accountID
	f.lastCategoryID = catID
	f.lastName = name
	return &Project{ID: "project-1", CategoryID: catID, Name: name, Status: "active", Public: true}, f.err
}

func (f *fakeStore) UpdateProject(accountID string, project *Project) (*Project, error) {
	f.lastAccountID = accountID
	f.project = project
	return project, f.err
}

func (f *fakeStore) DeleteProject(accountID string, id string) (*Project, error) {
	f.lastAccountID = accountID
	f.lastProjectID = id
	return &Project{ID: id}, f.err
}

func (f *fakeStore) MoveProject(accountID string, projectID string, targetCategoryID string, targetIndex int) (*Project, error) {
	f.lastAccountID = accountID
	f.lastProjectID = projectID
	f.lastTargetCategoryID = targetCategoryID
	f.lastTargetIndex = targetIndex
	return &Project{ID: projectID, CategoryID: targetCategoryID}, f.err
}

func (f *fakeStore) ArchiveProject(accountID string, id string) (*Project, error) {
	f.lastAccountID = accountID
	f.lastProjectID = id
	return &Project{ID: id, Status: "archived"}, f.err
}

func (f *fakeStore) RestoreProject(accountID string, id string) (*Project, error) {
	f.lastAccountID = accountID
	f.lastProjectID = id
	return &Project{ID: id, Status: "active"}, f.err
}

func (f *fakeStore) ReorderProjects(accountID string, catID string, projectIDs []string) error {
	f.lastAccountID = accountID
	f.lastCategoryID = catID
	f.lastProjectIDs = projectIDs
	return f.err
}

func (f *fakeStore) GetTask(accountID string, id string) (*Task, error) {
	f.lastAccountID = accountID
	f.lastTaskID = id
	if f.task != nil {
		return f.task, f.err
	}
	return &Task{ID: id, Status: "active", Public: true, ParentPublic: true}, f.err
}

func (f *fakeStore) AddTask(accountID string, projectID string, name string) (*Task, error) {
	f.lastAccountID = accountID
	f.lastProjectID = projectID
	f.lastName = name
	return &Task{ID: "task-1", ProjectID: projectID, Name: name, Status: "active", Public: true}, f.err
}

func (f *fakeStore) UpdateTask(accountID string, task *Task) (*Task, error) {
	f.lastAccountID = accountID
	f.task = task
	return task, f.err
}

func (f *fakeStore) DeleteTask(accountID string, id string) (*Task, error) {
	f.lastAccountID = accountID
	f.lastTaskID = id
	return &Task{ID: id}, f.err
}

func (f *fakeStore) MoveTask(accountID string, taskID string, targetProjectID string, targetIndex int) (*Task, error) {
	f.lastAccountID = accountID
	f.lastTaskID = taskID
	f.lastTargetProjectID = targetProjectID
	f.lastTargetIndex = targetIndex
	return &Task{ID: taskID, ProjectID: targetProjectID}, f.err
}

func (f *fakeStore) ArchiveTask(accountID string, id string) (*Task, error) {
	f.lastAccountID = accountID
	f.lastTaskID = id
	return &Task{ID: id, Status: "archived"}, f.err
}

func (f *fakeStore) RestoreTask(accountID string, id string) (*Task, error) {
	f.lastAccountID = accountID
	f.lastTaskID = id
	return &Task{ID: id, Status: "active"}, f.err
}

func (f *fakeStore) ReorderTasks(accountID string, projectID string, taskIDs []string) error {
	f.lastAccountID = accountID
	f.lastProjectID = projectID
	f.lastTaskIDs = taskIDs
	return f.err
}

func (f *fakeStore) AddWorkLogForProject(accountID string, projectID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error) {
	f.lastAccountID = accountID
	f.lastProjectID = projectID
	return &WorkLog{ID: "work-log-1", ProjectID: projectID, HoursWorked: hoursWorked, WorkDescription: workDescription, CompletionEstimate: completionEstimate}, f.err
}

func (f *fakeStore) AddWorkLogForTask(accountID string, taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*WorkLog, error) {
	f.lastAccountID = accountID
	f.lastTaskID = taskID
	return &WorkLog{ID: "work-log-1", TaskID: taskID, HoursWorked: hoursWorked, WorkDescription: workDescription, CompletionEstimate: completionEstimate}, f.err
}

func (f *fakeStore) GetWorkLogsForTask(accountID string, taskID string) ([]*WorkLog, error) {
	return nil, f.err
}

func (f *fakeStore) GetWorkLogsForProject(accountID string, projectID string) ([]*WorkLog, error) {
	return nil, f.err
}

func (f *fakeStore) GetWorkLogsForCategory(accountID string, categoryID string) ([]*WorkLog, error) {
	return nil, f.err
}

func newTestService(t *testing.T, store *fakeStore) *Service {
	t.Helper()
	svc, err := New(Options{
		Store: store,
		Clock: func() time.Time {
			return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func TestNew_ValidatesRequiredDependencies(t *testing.T) {
	if _, err := New(Options{Clock: time.Now}); err == nil {
		t.Fatal("New without store succeeded; want error")
	}
	if _, err := New(Options{Store: &fakeStore{}}); err == nil {
		t.Fatal("New without clock succeeded; want error")
	}
	if _, err := New(Options{Store: &fakeStore{}, Clock: time.Now}); err != nil {
		t.Fatalf("New with required dependencies returned error: %v", err)
	}
}

func TestService_ValidatesCreateAndUpdateInputs(t *testing.T) {
	svc := newTestService(t, &fakeStore{})

	if _, err := svc.CreateCategory(CreateCategoryInput{AccountID: " account-1 ", Name: "  "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateCategory error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateProject(CreateProjectInput{AccountID: "account-1", CategoryID: "", Name: "Project"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateProject error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateTask(CreateTaskInput{AccountID: "account-1", ProjectID: "project-1", Name: ""}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateTask error = %v, want ErrInvalidInput", err)
	}

	invalidCompletion := 101
	if _, err := svc.UpdateProject(UpdateProjectInput{AccountID: "account-1", ID: "project-1", Completion: &invalidCompletion}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateProject error = %v, want ErrInvalidInput", err)
	}
	invalidStatus := "paused"
	if _, err := svc.UpdateTask(UpdateTaskInput{AccountID: "account-1", ID: "task-1", Status: &invalidStatus}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateTask error = %v, want ErrInvalidInput", err)
	}
}

func TestService_NormalizesAndPassesValidUpdateToStore(t *testing.T) {
	store := &fakeStore{project: &Project{ID: "project-1", Status: "active", Public: true}}
	svc := newTestService(t, store)
	name := "  Renamed  "

	project, err := svc.UpdateProject(UpdateProjectInput{AccountID: " account-1 ", ID: " project-1 ", Name: &name})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if store.lastAccountID != "account-1" || project.ID != "project-1" || project.Name != "Renamed" {
		t.Fatalf("updated project/store = %#v/%#v", project, store)
	}
}

func TestService_ValidatesMoveInputs(t *testing.T) {
	svc := newTestService(t, &fakeStore{})

	if _, err := svc.MoveProject(MoveProjectInput{AccountID: "account-1", ID: "project-1", TargetCategoryID: "category-1", TargetIndex: -1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("MoveProject error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.MoveTask(MoveTaskInput{AccountID: "account-1", ID: "task-1", TargetProjectID: "", TargetIndex: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("MoveTask error = %v, want ErrInvalidInput", err)
	}
}

func TestService_PassesValidMoveToStore(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)

	if _, err := svc.MoveProject(MoveProjectInput{AccountID: " account-1 ", ID: " project-1 ", TargetCategoryID: " category-1 ", TargetIndex: 2}); err != nil {
		t.Fatalf("MoveProject: %v", err)
	}
	if store.lastAccountID != "account-1" || store.lastProjectID != "project-1" || store.lastTargetCategoryID != "category-1" || store.lastTargetIndex != 2 {
		t.Fatalf("move project store call = %#v", store)
	}

	if _, err := svc.MoveTask(MoveTaskInput{AccountID: " account-1 ", ID: " task-1 ", TargetProjectID: " project-2 ", TargetIndex: 1}); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if store.lastTaskID != "task-1" || store.lastTargetProjectID != "project-2" || store.lastTargetIndex != 1 {
		t.Fatalf("move task store call = %#v", store)
	}
}

func TestService_ValidatesArchiveAndRestoreInputs(t *testing.T) {
	svc := newTestService(t, &fakeStore{})

	if _, err := svc.ArchiveCategory(ArchiveCategoryInput{AccountID: "", ID: "category-1"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ArchiveCategory error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.RestoreProject(RestoreProjectInput{AccountID: "account-1", ID: ""}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RestoreProject error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.ArchiveTask(ArchiveTaskInput{AccountID: " account-1 ", ID: " task-1 "}); err != nil {
		t.Fatalf("ArchiveTask valid input returned error: %v", err)
	}
}

func TestService_MapsNotActiveStoreErrorsToInvalidInput(t *testing.T) {
	svc := newTestService(t, &fakeStore{err: errors.New("task parent is not active")})

	if _, err := svc.RestoreTask(RestoreTaskInput{AccountID: "account-1", ID: "task-1"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RestoreTask error = %v, want ErrInvalidInput", err)
	}
}

func TestService_ValidatesWorkLogInputs(t *testing.T) {
	svc := newTestService(t, &fakeStore{})

	if _, err := svc.AddProjectWorkLog(AddProjectWorkLogInput{AccountID: "account-1", ProjectID: "project-1", HoursWorked: -1, CompletionEstimate: 50}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AddProjectWorkLog negative hours error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.AddTaskWorkLog(AddTaskWorkLogInput{AccountID: "account-1", TaskID: "task-1", HoursWorked: 1, CompletionEstimate: 101}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AddTaskWorkLog invalid completion error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.AddTaskWorkLog(AddTaskWorkLogInput{AccountID: "account-1", TaskID: "", HoursWorked: 1, CompletionEstimate: 50}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AddTaskWorkLog missing task error = %v, want ErrInvalidInput", err)
	}

	store := &fakeStore{}
	svc = newTestService(t, store)
	if _, err := svc.AddProjectWorkLog(AddProjectWorkLogInput{AccountID: " account-1 ", ProjectID: " project-1 ", HoursWorked: 1, WorkDescription: " work ", CompletionEstimate: 50}); err != nil {
		t.Fatalf("AddProjectWorkLog valid input returned error: %v", err)
	}
	if store.lastAccountID != "account-1" || store.lastProjectID != "project-1" {
		t.Fatalf("work log store call = %#v", store)
	}
}

func TestService_FiltersPublicCategoryTree(t *testing.T) {
	store := &fakeStore{
		categories: []*Category{
			{
				ID:     "private-category",
				Public: false,
			},
			{
				ID:     "public-category",
				Public: true,
				Projects: []*Project{
					{
						ID:     "private-project",
						Public: false,
						Tasks:  []*Task{{ID: "leaked-task", Public: true}},
					},
					{
						ID:     "public-project",
						Public: true,
						Tasks: []*Task{
							{ID: "private-task", Public: false},
							{ID: "public-task", Public: true},
						},
					},
				},
			},
		},
	}
	svc := newTestService(t, store)

	cats, err := svc.ListCategories(ListCategoriesInput{AccountID: "account-1", Viewer: PublicViewer()})
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) != 1 || cats[0].ID != "public-category" {
		t.Fatalf("filtered categories = %#v, want public category only", cats)
	}
	if len(cats[0].Projects) != 1 || cats[0].Projects[0].ID != "public-project" {
		t.Fatalf("filtered projects = %#v, want public project only", cats[0].Projects)
	}
	if len(cats[0].Projects[0].Tasks) != 1 || cats[0].Projects[0].Tasks[0].ID != "public-task" {
		t.Fatalf("filtered tasks = %#v, want public task only", cats[0].Projects[0].Tasks)
	}
}

func TestService_ViewerChecksReturnNotFound(t *testing.T) {
	svc := newTestService(t, &fakeStore{
		category: &Category{ID: "category-1", Public: false},
		project:  &Project{ID: "project-1", Public: true, ParentPublic: false},
		task:     &Task{ID: "task-1", Public: true, ParentPublic: false},
	})

	if _, err := svc.GetCategory(GetCategoryInput{AccountID: "account-1", ID: "category-1", Viewer: PublicViewer()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetCategory error = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetProject(GetProjectInput{AccountID: "account-1", ID: "project-1", Viewer: PublicViewer()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProject error = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetTask(GetTaskInput{AccountID: "account-1", ID: "task-1", Viewer: PublicViewer()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetTask error = %v, want ErrNotFound", err)
	}
}

func TestMapStoreError(t *testing.T) {
	if err := mapStoreError(sql.ErrNoRows); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sql.ErrNoRows mapped to %v, want ErrNotFound", err)
	}
	if err := mapStoreError(errors.New("project parent is not active")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("not active error mapped to %v, want ErrInvalidInput", err)
	}
	other := errors.New("database unavailable")
	if err := mapStoreError(other); !errors.Is(err, other) {
		t.Fatalf("other error mapped to %v, want original", err)
	}
}
