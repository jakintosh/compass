package database_test

import (
	"testing"
	"time"
)

func TestAddProjectLog_CreatesLogAndUpdatesProjectStatus(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	createdAt := fixedTime(0)

	projectLog, err := db.AddProjectLog(
		accountID,
		projectID,
		30,
		"low",
		"Project feels early",
		&createdAt,
	)
	if err != nil {
		t.Fatalf("add project log: %v", err)
	}
	if projectLog.ProjectID != projectID {
		t.Fatalf("project log project ID = %q, want %q", projectLog.ProjectID, projectID)
	}
	if !projectLog.CreatedAt.Equal(createdAt) {
		t.Fatalf("created at = %s, want %s", projectLog.CreatedAt, createdAt)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project.Completion != 30 || project.Confidence != "low" {
		t.Fatalf("project status/confidence = %d/%q, want 30/low", project.Completion, project.Confidence)
	}

	logs, err := db.GetProjectLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != projectLog.ID {
		t.Fatalf("project logs = %#v, want inserted project log %q", logs, projectLog.ID)
	}
}

func TestAddTaskLog_CreatesLogAndUpdatesTaskCompletionOnly(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	taskLog, err := db.AddTaskLog(
		accountID,
		taskID,
		2.25,
		"Task work",
		55,
		&createdAt,
	)
	if err != nil {
		t.Fatalf("add task log: %v", err)
	}
	if taskLog.ProjectID != projectID || taskLog.TaskID != taskID {
		t.Fatalf("task log project/task IDs = %q/%q, want %q/%q", taskLog.ProjectID, taskLog.TaskID, projectID, taskID)
	}

	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Completion != 55 {
		t.Fatalf("task completion = %d, want 55", task.Completion)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project.Completion != 0 {
		t.Fatalf("project completion = %d, want unchanged 0", project.Completion)
	}

	logs, err := db.GetTaskLogsForTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != taskLog.ID {
		t.Fatalf("task logs = %#v, want inserted task log %q", logs, taskLog.ID)
	}
}

func TestAddLog_UsesCurrentTimeWhenCustomTimeIsNil(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	before := time.Now().Add(-time.Second).Unix()
	projectLog, err := db.AddProjectLog(
		accountID,
		projectID,
		10,
		"medium",
		"Project check-in",
		nil,
	)
	after := time.Now().Add(time.Second).Unix()
	if err != nil {
		t.Fatalf("add project log: %v", err)
	}

	got := projectLog.CreatedAt.Unix()
	if got < before || got > after {
		t.Fatalf("created at unix = %d, want between %d and %d", got, before, after)
	}
}

func TestGetLogs_FilterAndSortNewestFirst(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	otherCategoryID := seedCategory(t, db, accountID, "Other Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	otherProjectID := seedProject(t, db, accountID, otherCategoryID, "Other Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	older := fixedTime(0)
	newer := fixedTime(1)
	otherTime := fixedTime(2)

	projectLog, err := db.AddProjectLog(accountID, projectID, 20, "medium", "Project check-in", &older)
	if err != nil {
		t.Fatalf("add project log: %v", err)
	}
	taskLog, err := db.AddTaskLog(accountID, taskID, 1, "Task work", 40, &newer)
	if err != nil {
		t.Fatalf("add task log: %v", err)
	}
	if _, err := db.AddProjectLog(accountID, otherProjectID, 60, "high", "Other check-in", &otherTime); err != nil {
		t.Fatalf("add other project log: %v", err)
	}

	projectLogs, err := db.GetProjectLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project logs: %v", err)
	}
	if len(projectLogs) != 1 || projectLogs[0].ID != projectLog.ID {
		t.Fatalf("project logs = %#v, want project log %q", projectLogs, projectLog.ID)
	}

	projectTaskLogs, err := db.GetTaskLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project task logs: %v", err)
	}
	if len(projectTaskLogs) != 1 || projectTaskLogs[0].ID != taskLog.ID {
		t.Fatalf("project task logs = %#v, want task log %q", projectTaskLogs, taskLog.ID)
	}

	taskLogs, err := db.GetTaskLogsForTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task logs: %v", err)
	}
	if len(taskLogs) != 1 || taskLogs[0].ID != taskLog.ID {
		t.Fatalf("task logs = %#v, want task log %q", taskLogs, taskLog.ID)
	}

	categoryTaskLogs, err := db.GetTaskLogsForCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category task logs: %v", err)
	}
	if len(categoryTaskLogs) != 1 || categoryTaskLogs[0].ID != taskLog.ID {
		t.Fatalf("category task logs = %#v, want task log %q", categoryTaskLogs, taskLog.ID)
	}
}

func TestLogs_ScopeByAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	categoryID := seedCategory(t, db, aliceID, "Category")
	projectID := seedProject(t, db, aliceID, categoryID, "Project")
	createdAt := fixedTime(0)

	if _, err := db.AddProjectLog(aliceID, projectID, 25, "medium", "Project check-in", &createdAt); err != nil {
		t.Fatalf("add project log: %v", err)
	}

	logs, err := db.GetProjectLogsForProject(bobID, projectID)
	if err != nil {
		t.Fatalf("get bob project logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("bob project logs = %#v, want none", logs)
	}
}

func TestAddProjectLog_MissingProjectDoesNotCreateLog(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")

	if _, err := db.AddProjectLog(accountID, "missing-project", 10, "medium", "Project check-in", nil); err == nil {
		t.Fatal("AddProjectLog with missing project succeeded; want error")
	}

	logs, err := db.GetProjectLogsForProject(accountID, "missing-project")
	if err != nil {
		t.Fatalf("get missing project logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("missing project logs = %#v, want none", logs)
	}
}

func TestTaskLogs_FollowCurrentAttributionAfterTaskMove(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	sourceProjectID := seedProject(t, db, accountID, categoryID, "Source")
	targetProjectID := seedProject(t, db, accountID, categoryID, "Target")
	taskID := seedTask(t, db, accountID, sourceProjectID, "Task")
	createdAt := fixedTime(0)

	taskLog, err := db.AddTaskLog(accountID, taskID, 1, "Task work", 25, &createdAt)
	if err != nil {
		t.Fatalf("add task log: %v", err)
	}
	if _, err := db.MoveTask(accountID, taskID, targetProjectID, 0); err != nil {
		t.Fatalf("move task: %v", err)
	}

	sourceLogs, err := db.GetTaskLogsForProject(accountID, sourceProjectID)
	if err != nil {
		t.Fatalf("get source project task logs: %v", err)
	}
	if len(sourceLogs) != 0 {
		t.Fatalf("source project task logs = %#v, want none", sourceLogs)
	}

	targetLogs, err := db.GetTaskLogsForProject(accountID, targetProjectID)
	if err != nil {
		t.Fatalf("get target project task logs: %v", err)
	}
	if len(targetLogs) != 1 || targetLogs[0].ID != taskLog.ID {
		t.Fatalf("target project task logs = %#v, want moved task log %q", targetLogs, taskLog.ID)
	}
	if targetLogs[0].ProjectID != targetProjectID {
		t.Fatalf("derived task log project ID = %q, want %q", targetLogs[0].ProjectID, targetProjectID)
	}
}

func TestLogs_FollowCurrentAttributionAfterProjectMove(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	sourceCategoryID := seedCategory(t, db, accountID, "Source")
	targetCategoryID := seedCategory(t, db, accountID, "Target")
	projectID := seedProject(t, db, accountID, sourceCategoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	if _, err := db.AddProjectLog(accountID, projectID, 25, "medium", "Project check-in", &createdAt); err != nil {
		t.Fatalf("add project log: %v", err)
	}
	taskLog, err := db.AddTaskLog(accountID, taskID, 1, "Task work", 50, &createdAt)
	if err != nil {
		t.Fatalf("add task log: %v", err)
	}
	if _, err := db.MoveProject(accountID, projectID, targetCategoryID, 0); err != nil {
		t.Fatalf("move project: %v", err)
	}

	sourceLogs, err := db.GetTaskLogsForCategory(accountID, sourceCategoryID)
	if err != nil {
		t.Fatalf("get source category task logs: %v", err)
	}
	if len(sourceLogs) != 0 {
		t.Fatalf("source category task logs = %#v, want none", sourceLogs)
	}

	targetLogs, err := db.GetTaskLogsForCategory(accountID, targetCategoryID)
	if err != nil {
		t.Fatalf("get target category task logs: %v", err)
	}
	if len(targetLogs) != 1 || targetLogs[0].ID != taskLog.ID {
		t.Fatalf("target category task logs = %#v, want task log %q", targetLogs, taskLog.ID)
	}
	if targetLogs[0].CategoryID != targetCategoryID {
		t.Fatalf("derived task log category ID = %q, want %q", targetLogs[0].CategoryID, targetCategoryID)
	}
}

func TestLogs_RemainQueryableWhenParentIsArchived(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	projectLog, err := db.AddProjectLog(accountID, projectID, 25, "medium", "Project check-in", &createdAt)
	if err != nil {
		t.Fatalf("add project log: %v", err)
	}
	taskLog, err := db.AddTaskLog(accountID, taskID, 1, "Task work", 50, &createdAt)
	if err != nil {
		t.Fatalf("add task log: %v", err)
	}
	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	projectLogs, err := db.GetProjectLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project logs: %v", err)
	}
	if len(projectLogs) != 1 || projectLogs[0].ID != projectLog.ID {
		t.Fatalf("project logs = %#v, want project log %q", projectLogs, projectLog.ID)
	}

	taskLogs, err := db.GetTaskLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project task logs: %v", err)
	}
	if len(taskLogs) != 1 || taskLogs[0].ID != taskLog.ID {
		t.Fatalf("project task logs = %#v, want task log %q", taskLogs, taskLog.ID)
	}
}
