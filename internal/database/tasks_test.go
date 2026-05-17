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

func TestMoveTask_ChangesProjectCategoryAndTargetOrdering(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	sourceCategoryID := seedCategory(t, db, accountID, "Source Category")
	targetCategoryID := seedCategory(t, db, accountID, "Target Category")
	sourceProjectID := seedProject(t, db, accountID, sourceCategoryID, "Source Project")
	targetProjectID := seedProject(t, db, accountID, targetCategoryID, "Target Project")
	movedTaskID := seedTask(t, db, accountID, sourceProjectID, "Moved")
	firstTargetID := seedTask(t, db, accountID, targetProjectID, "First Target")
	secondTargetID := seedTask(t, db, accountID, targetProjectID, "Second Target")

	moved, err := db.MoveTask(accountID, movedTaskID, targetProjectID, 1)
	if err != nil {
		t.Fatalf("move task: %v", err)
	}
	if moved.ProjectID != targetProjectID || moved.CategoryID != targetCategoryID {
		t.Fatalf("moved task project/category = %q/%q, want %q/%q", moved.ProjectID, moved.CategoryID, targetProjectID, targetCategoryID)
	}

	source, err := db.GetProject(accountID, sourceProjectID)
	if err != nil {
		t.Fatalf("get source project: %v", err)
	}
	if len(source.Tasks) != 0 {
		t.Fatalf("source tasks = %#v, want none", source.Tasks)
	}

	target, err := db.GetProject(accountID, targetProjectID)
	if err != nil {
		t.Fatalf("get target project: %v", err)
	}
	got := []string{target.Tasks[0].ID, target.Tasks[1].ID, target.Tasks[2].ID}
	want := []string{firstTargetID, movedTaskID, secondTargetID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("target order = %#v, want %#v", got, want)
		}
	}
}

func TestMoveTask_ReordersWithinSameProject(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	firstID := seedTask(t, db, accountID, projectID, "First")
	secondID := seedTask(t, db, accountID, projectID, "Second")
	thirdID := seedTask(t, db, accountID, projectID, "Third")

	if _, err := db.MoveTask(accountID, thirdID, projectID, 0); err != nil {
		t.Fatalf("move task within project: %v", err)
	}

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	got := []string{project.Tasks[0].ID, project.Tasks[1].ID, project.Tasks[2].ID}
	want := []string{thirdID, firstID, secondID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("task order = %#v, want %#v", got, want)
		}
	}
}

func TestMoveTask_ClampsLargeTargetIndexToEnd(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	sourceProjectID := seedProject(t, db, accountID, categoryID, "Source")
	targetProjectID := seedProject(t, db, accountID, categoryID, "Target")
	movedID := seedTask(t, db, accountID, sourceProjectID, "Moved")
	firstTargetID := seedTask(t, db, accountID, targetProjectID, "First Target")

	if _, err := db.MoveTask(accountID, movedID, targetProjectID, 99); err != nil {
		t.Fatalf("move task: %v", err)
	}

	target, err := db.GetProject(accountID, targetProjectID)
	if err != nil {
		t.Fatalf("get target project: %v", err)
	}
	got := []string{target.Tasks[0].ID, target.Tasks[1].ID}
	want := []string{firstTargetID, movedID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("task order = %#v, want %#v", got, want)
		}
	}
}

func TestMoveTask_RequiresTargetProjectInSameAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	aliceCategoryID := seedCategory(t, db, aliceID, "Alice Category")
	bobCategoryID := seedCategory(t, db, bobID, "Bob Category")
	aliceProjectID := seedProject(t, db, aliceID, aliceCategoryID, "Alice Project")
	bobProjectID := seedProject(t, db, bobID, bobCategoryID, "Bob Project")
	taskID := seedTask(t, db, aliceID, aliceProjectID, "Task")

	if _, err := db.MoveTask(aliceID, taskID, bobProjectID, 0); err == nil {
		t.Fatal("MoveTask to another account's project succeeded; want error")
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
