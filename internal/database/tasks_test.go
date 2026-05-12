package database_test

import "testing"

func TestAddTask_RequiresProjectInSameAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	bobCategoryID := seedCategory(t, db, bobID, "Bob Category")
	bobProjectID := seedProject(t, db, bobID, bobCategoryID, "Bob Project")

	if _, err := db.AddTask(aliceID, bobProjectID, "Task"); err == nil {
		t.Fatal("AddTask with another account's project succeeded; want error")
	}
}

func TestGetTask_ReturnsParentVisibility(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	project.Public = false
	if _, err := db.UpdateProject(accountID, project); err != nil {
		t.Fatalf("update project visibility: %v", err)
	}

	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ID != taskID || task.ProjectID != projectID || task.CategoryID != categoryID {
		t.Fatalf("task = %#v, want task %q in project %q/category %q", task, taskID, projectID, categoryID)
	}
	if task.ParentPublic {
		t.Fatal("task ParentPublic = true, want false after project made private")
	}
}

func TestUpdateTask_PersistsFields(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	task, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	task.Name = "Renamed"
	task.Description = "Updated description"
	task.Completion = 64
	task.Public = false

	updated, err := db.UpdateTask(accountID, task)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if updated.Name != "Renamed" || updated.Description != "Updated description" || updated.Completion != 64 || updated.Public {
		t.Fatalf("updated task = %#v, want renamed private task at 64", updated)
	}

	readBack, err := db.GetTask(accountID, taskID)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if readBack.Name != "Renamed" || readBack.Description != "Updated description" || readBack.Completion != 64 || readBack.Public {
		t.Fatalf("read task = %#v, want renamed private task at 64", readBack)
	}
}

func TestDeleteTask_RemovesTask(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.DeleteTask(accountID, taskID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if _, err := db.GetTask(accountID, taskID); err == nil {
		t.Fatal("GetTask succeeded after delete; want error")
	}
}

func TestReorderTasks_ChangesOnlyProjectOrdering(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	otherProjectID := seedProject(t, db, accountID, categoryID, "Other Project")
	firstTaskID := seedTask(t, db, accountID, projectID, "First")
	secondTaskID := seedTask(t, db, accountID, projectID, "Second")
	otherTaskID := seedTask(t, db, accountID, otherProjectID, "Other")

	if err := db.ReorderTasks(accountID, projectID, []string{secondTaskID, firstTaskID}); err != nil {
		t.Fatalf("reorder tasks: %v", err)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project.Tasks[0].ID != secondTaskID || project.Tasks[1].ID != firstTaskID {
		t.Fatalf("task order = [%q, %q], want [%q, %q]", project.Tasks[0].ID, project.Tasks[1].ID, secondTaskID, firstTaskID)
	}

	otherProject, err := db.GetProject(accountID, otherProjectID)
	if err != nil {
		t.Fatalf("get other project: %v", err)
	}
	if len(otherProject.Tasks) != 1 || otherProject.Tasks[0].ID != otherTaskID {
		t.Fatalf("other project tasks = %#v, want unchanged task %q", otherProject.Tasks, otherTaskID)
	}
}

func TestGetTask_ScopesByAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	categoryID := seedCategory(t, db, aliceID, "Category")
	projectID := seedProject(t, db, aliceID, categoryID, "Project")
	taskID := seedTask(t, db, aliceID, projectID, "Task")

	if _, err := db.GetTask(bobID, taskID); err == nil {
		t.Fatal("GetTask with another account succeeded; want error")
	}
}
