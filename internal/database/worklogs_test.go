package database_test

import (
	"testing"
	"time"
)

func TestAddWorkLogForProject_CreatesLogAndUpdatesCompletion(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	createdAt := fixedTime(0)

	workLog, err := db.AddWorkLogForProject(
		accountID,
		projectID,
		1.5,
		"Project work",
		30,
		&createdAt,
	)
	if err != nil {
		t.Fatalf("add project work log: %v", err)
	}
	if workLog.ProjectID != projectID || workLog.TaskID != "" {
		t.Fatalf("work log project/task IDs = %q/%q, want %q/empty", workLog.ProjectID, workLog.TaskID, projectID)
	}
	if !workLog.CreatedAt.Equal(createdAt) {
		t.Fatalf("created at = %s, want %s", workLog.CreatedAt, createdAt)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project.Completion != 30 {
		t.Fatalf("project completion = %d, want 30", project.Completion)
	}

	logs, err := db.GetWorkLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project work logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != workLog.ID {
		t.Fatalf("project work logs = %#v, want inserted work log %q", logs, workLog.ID)
	}
}

func TestAddWorkLogForTask_CreatesLogAndUpdatesCompletion(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	workLog, err := db.AddWorkLogForTask(
		accountID,
		taskID,
		2.25,
		"Task work",
		55,
		&createdAt,
	)
	if err != nil {
		t.Fatalf("add task work log: %v", err)
	}
	if workLog.ProjectID != projectID || workLog.TaskID != taskID {
		t.Fatalf("work log project/task IDs = %q/%q, want %q/%q", workLog.ProjectID, workLog.TaskID, projectID, taskID)
	}

	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Completion != 55 {
		t.Fatalf("task completion = %d, want 55", task.Completion)
	}

	logs, err := db.GetWorkLogsForTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task work logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != workLog.ID {
		t.Fatalf("task work logs = %#v, want inserted work log %q", logs, workLog.ID)
	}
}

func TestAddWorkLog_UsesCurrentTimeWhenCustomTimeIsNil(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	before := time.Now().Add(-time.Second).Unix()
	workLog, err := db.AddWorkLogForProject(
		accountID,
		projectID,
		1,
		"Project work",
		10,
		nil,
	)
	after := time.Now().Add(time.Second).Unix()
	if err != nil {
		t.Fatalf("add project work log: %v", err)
	}

	got := workLog.CreatedAt.Unix()
	if got < before || got > after {
		t.Fatalf("created at unix = %d, want between %d and %d", got, before, after)
	}
}

func TestGetWorkLogs_FiltersAndSortsNewestFirst(t *testing.T) {
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

	projectLog, err := db.AddWorkLogForProject(accountID, projectID, 1, "Project work", 20, &older)
	if err != nil {
		t.Fatalf("add project work log: %v", err)
	}
	taskLog, err := db.AddWorkLogForTask(accountID, taskID, 1, "Task work", 40, &newer)
	if err != nil {
		t.Fatalf("add task work log: %v", err)
	}
	if _, err := db.AddWorkLogForProject(accountID, otherProjectID, 1, "Other work", 60, &otherTime); err != nil {
		t.Fatalf("add other project work log: %v", err)
	}

	projectLogs, err := db.GetWorkLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project work logs: %v", err)
	}
	if len(projectLogs) != 2 {
		t.Fatalf("project log count = %d, want 2", len(projectLogs))
	}
	if projectLogs[0].ID != taskLog.ID || projectLogs[1].ID != projectLog.ID {
		t.Fatalf("project log order = [%q, %q], want [%q, %q]", projectLogs[0].ID, projectLogs[1].ID, taskLog.ID, projectLog.ID)
	}

	taskLogs, err := db.GetWorkLogsForTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task work logs: %v", err)
	}
	if len(taskLogs) != 1 || taskLogs[0].ID != taskLog.ID {
		t.Fatalf("task logs = %#v, want task log %q", taskLogs, taskLog.ID)
	}

	categoryLogs, err := db.GetWorkLogsForCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category work logs: %v", err)
	}
	if len(categoryLogs) != 2 {
		t.Fatalf("category log count = %d, want 2", len(categoryLogs))
	}
	if categoryLogs[0].ID != taskLog.ID || categoryLogs[1].ID != projectLog.ID {
		t.Fatalf("category log order = [%q, %q], want [%q, %q]", categoryLogs[0].ID, categoryLogs[1].ID, taskLog.ID, projectLog.ID)
	}
}

func TestWorkLogs_ScopeByAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	categoryID := seedCategory(t, db, aliceID, "Category")
	projectID := seedProject(t, db, aliceID, categoryID, "Project")
	createdAt := fixedTime(0)

	if _, err := db.AddWorkLogForProject(aliceID, projectID, 1, "Project work", 25, &createdAt); err != nil {
		t.Fatalf("add project work log: %v", err)
	}

	logs, err := db.GetWorkLogsForProject(bobID, projectID)
	if err != nil {
		t.Fatalf("get bob project work logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("bob project logs = %#v, want none", logs)
	}
}

func TestAddWorkLogForProject_MissingProjectDoesNotCreateLog(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")

	if _, err := db.AddWorkLogForProject(accountID, "missing-project", 1, "Project work", 10, nil); err == nil {
		t.Fatal("AddWorkLogForProject with missing project succeeded; want error")
	}

	logs, err := db.GetWorkLogsForProject(accountID, "missing-project")
	if err != nil {
		t.Fatalf("get missing project logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("missing project logs = %#v, want none", logs)
	}
}

func TestWorkLogs_FollowCurrentAttributionAfterTaskMove(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	sourceProjectID := seedProject(t, db, accountID, categoryID, "Source")
	targetProjectID := seedProject(t, db, accountID, categoryID, "Target")
	taskID := seedTask(t, db, accountID, sourceProjectID, "Task")
	createdAt := fixedTime(0)

	workLog, err := db.AddWorkLogForTask(accountID, taskID, 1, "Task work", 25, &createdAt)
	if err != nil {
		t.Fatalf("add task work log: %v", err)
	}
	if _, err := db.MoveTask(accountID, taskID, targetProjectID, 0); err != nil {
		t.Fatalf("move task: %v", err)
	}

	sourceLogs, err := db.GetWorkLogsForProject(accountID, sourceProjectID)
	if err != nil {
		t.Fatalf("get source project logs: %v", err)
	}
	if len(sourceLogs) != 0 {
		t.Fatalf("source project logs = %#v, want none", sourceLogs)
	}

	targetLogs, err := db.GetWorkLogsForProject(accountID, targetProjectID)
	if err != nil {
		t.Fatalf("get target project logs: %v", err)
	}
	if len(targetLogs) != 1 || targetLogs[0].ID != workLog.ID {
		t.Fatalf("target project logs = %#v, want moved task log %q", targetLogs, workLog.ID)
	}
	if targetLogs[0].ProjectID != targetProjectID {
		t.Fatalf("derived work log project ID = %q, want %q", targetLogs[0].ProjectID, targetProjectID)
	}
}

func TestWorkLogs_FollowCurrentAttributionAfterProjectMove(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	sourceCategoryID := seedCategory(t, db, accountID, "Source")
	targetCategoryID := seedCategory(t, db, accountID, "Target")
	projectID := seedProject(t, db, accountID, sourceCategoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	projectLog, err := db.AddWorkLogForProject(accountID, projectID, 1, "Project work", 25, &createdAt)
	if err != nil {
		t.Fatalf("add project work log: %v", err)
	}
	taskLog, err := db.AddWorkLogForTask(accountID, taskID, 1, "Task work", 50, &createdAt)
	if err != nil {
		t.Fatalf("add task work log: %v", err)
	}
	if _, err := db.MoveProject(accountID, projectID, targetCategoryID, 0); err != nil {
		t.Fatalf("move project: %v", err)
	}

	sourceLogs, err := db.GetWorkLogsForCategory(accountID, sourceCategoryID)
	if err != nil {
		t.Fatalf("get source category logs: %v", err)
	}
	if len(sourceLogs) != 0 {
		t.Fatalf("source category logs = %#v, want none", sourceLogs)
	}

	targetLogs, err := db.GetWorkLogsForCategory(accountID, targetCategoryID)
	if err != nil {
		t.Fatalf("get target category logs: %v", err)
	}
	if len(targetLogs) != 2 {
		t.Fatalf("target category log count = %d, want 2", len(targetLogs))
	}
	got := map[string]bool{targetLogs[0].ID: true, targetLogs[1].ID: true}
	if !got[projectLog.ID] || !got[taskLog.ID] {
		t.Fatalf("target category logs = %#v, want project log %q and task log %q", targetLogs, projectLog.ID, taskLog.ID)
	}
	for _, log := range targetLogs {
		if log.CategoryID != targetCategoryID {
			t.Fatalf("derived work log category ID = %q, want %q", log.CategoryID, targetCategoryID)
		}
	}
}

func TestWorkLogs_RemainQueryableWhenParentIsArchived(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")
	createdAt := fixedTime(0)

	projectLog, err := db.AddWorkLogForProject(accountID, projectID, 1, "Project work", 25, &createdAt)
	if err != nil {
		t.Fatalf("add project work log: %v", err)
	}
	taskLog, err := db.AddWorkLogForTask(accountID, taskID, 1, "Task work", 50, &createdAt)
	if err != nil {
		t.Fatalf("add task work log: %v", err)
	}
	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	projectLogs, err := db.GetWorkLogsForProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project work logs: %v", err)
	}
	if len(projectLogs) != 2 {
		t.Fatalf("project log count = %d, want 2", len(projectLogs))
	}
	got := map[string]bool{projectLogs[0].ID: true, projectLogs[1].ID: true}
	if !got[projectLog.ID] || !got[taskLog.ID] {
		t.Fatalf("project logs = %#v, want project log %q and task log %q", projectLogs, projectLog.ID, taskLog.ID)
	}
}
