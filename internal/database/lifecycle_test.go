package database_test

import (
	"encoding/json"
	"testing"
)

func TestArchiveCategory_DoesNotMutateChildrenButHidesThemFromList(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.ArchiveCategory(accountID, categoryID); err != nil {
		t.Fatalf("archive category: %v", err)
	}

	categories, err := db.GetCategories(accountID)
	if err != nil {
		t.Fatalf("get categories: %v", err)
	}
	if len(categories) != 0 {
		t.Fatalf("category count after archive = %d, want 0", len(categories))
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get child project after parent archive: %v", err)
	}
	if project.Status != "active" {
		t.Fatalf("child project status = %q, want active", project.Status)
	}
	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get child task after parent archive: %v", err)
	}
	if task.Status != "active" {
		t.Fatalf("child task status = %q, want active", task.Status)
	}

	if _, err := db.RestoreCategory(accountID, categoryID); err != nil {
		t.Fatalf("restore category: %v", err)
	}
	categories, err = db.GetCategories(accountID)
	if err != nil {
		t.Fatalf("get categories after restore: %v", err)
	}
	if len(categories) != 1 || len(categories[0].Projects) != 1 || len(categories[0].Projects[0].Tasks) != 1 {
		t.Fatalf("restored tree = %#v, want category/project/task visible", categories)
	}
}

func TestArchiveProject_HidesTasksWithoutMutatingTaskStatus(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	category, err := db.GetCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category: %v", err)
	}
	if len(category.Projects) != 0 {
		t.Fatalf("visible projects = %#v, want none after project archive", category.Projects)
	}

	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task after project archive: %v", err)
	}
	if task.Status != "active" {
		t.Fatalf("child task status = %q, want active", task.Status)
	}
}

func TestArchiveTask_HidesOnlyThatTask(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	archivedTaskID := seedTask(t, db, accountID, projectID, "Archived")
	activeTaskID := seedTask(t, db, accountID, projectID, "Active")

	if _, err := db.ArchiveTask(accountID, archivedTaskID); err != nil {
		t.Fatalf("archive task: %v", err)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != activeTaskID {
		t.Fatalf("visible tasks = %#v, want only active sibling %q", project.Tasks, activeTaskID)
	}
}

func TestRestoreCategory_DoesNotRevealExplicitlyArchivedProject(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}
	if _, err := db.ArchiveCategory(accountID, categoryID); err != nil {
		t.Fatalf("archive category: %v", err)
	}
	if _, err := db.RestoreCategory(accountID, categoryID); err != nil {
		t.Fatalf("restore category: %v", err)
	}

	category, err := db.GetCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category: %v", err)
	}
	if len(category.Projects) != 0 {
		t.Fatalf("visible projects = %#v, want none because project is explicitly archived", category.Projects)
	}
}

func TestRestoreProject_RequiresActiveCategory(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}
	if _, err := db.ArchiveCategory(accountID, categoryID); err != nil {
		t.Fatalf("archive category: %v", err)
	}
	if _, err := db.RestoreProject(accountID, projectID); err == nil {
		t.Fatal("restore project under archived category succeeded; want error")
	}
}

func TestRestoreTask_RequiresActiveParents(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.ArchiveTask(accountID, taskID); err != nil {
		t.Fatalf("archive task: %v", err)
	}
	if _, err := db.ArchiveProject(accountID, projectID); err != nil {
		t.Fatalf("archive project: %v", err)
	}
	if _, err := db.RestoreTask(accountID, taskID); err == nil {
		t.Fatal("restore task under archived project succeeded; want error")
	}
}

func TestEntityEvents_AreInsertedForCoreMutations(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.ArchiveTask(accountID, taskID); err != nil {
		t.Fatalf("archive task: %v", err)
	}
	if _, err := db.RestoreTask(accountID, taskID); err != nil {
		t.Fatalf("restore task: %v", err)
	}
	if _, err := db.AddWorkLogForTask(accountID, taskID, 1, "Task work", 50, nil); err != nil {
		t.Fatalf("add work log: %v", err)
	}

	for _, eventType := range []string{
		"category.created",
		"project.created",
		"task.created",
		"task.status_changed",
		"work_log.created",
	} {
		var count int
		if err := db.Conn.QueryRow(`
			SELECT COUNT(*)
			FROM entity_events
			WHERE account_id = ?1 AND event_type = ?2`,
			accountID,
			eventType,
		).Scan(&count); err != nil {
			t.Fatalf("count event %q: %v", eventType, err)
		}
		if count == 0 {
			t.Fatalf("event %q count = 0, want at least 1", eventType)
		}
	}
}

func TestEntityEvents_MoveProjectPayloadIncludesMoveContext(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	sourceCategoryID := seedCategory(t, db, accountID, "Source")
	targetCategoryID := seedCategory(t, db, accountID, "Target")
	projectID := seedProject(t, db, accountID, sourceCategoryID, "Project")

	if _, err := db.MoveProject(accountID, projectID, targetCategoryID, 2); err != nil {
		t.Fatalf("move project: %v", err)
	}

	var payload string
	if err := db.Conn.QueryRow(`
		SELECT data_json
		FROM entity_events
		WHERE account_id = ?1
			AND entity_type = 'project'
			AND entity_id = ?2
			AND event_type = 'project.moved'
		ORDER BY occurred_at DESC
		LIMIT 1`,
		accountID,
		projectID,
	).Scan(&payload); err != nil {
		t.Fatalf("read project move event: %v", err)
	}

	var data struct {
		FromCategoryID string `json:"from_category_id"`
		ToCategoryID   string `json:"to_category_id"`
		ToIndex        int    `json:"to_index"`
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("unmarshal event payload %q: %v", payload, err)
	}
	if data.FromCategoryID != sourceCategoryID ||
		data.ToCategoryID != targetCategoryID ||
		data.ToIndex != 2 {
		t.Fatalf("move event payload = %#v, want source %q target %q index 2", data, sourceCategoryID, targetCategoryID)
	}
}
