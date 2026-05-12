package database_test

import "testing"

func TestAddProject_RequiresCategoryInSameAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	bobCategoryID := seedCategory(t, db, bobID, "Bob Category")

	if _, err := db.AddProject(aliceID, bobCategoryID, "Project"); err == nil {
		t.Fatal("AddProject with another account's category succeeded; want error")
	}
}

func TestGetProject_ReturnsNestedTasks(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project.ID != projectID || project.CategoryID != categoryID {
		t.Fatalf("project = %#v, want project %q in category %q", project, projectID, categoryID)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != taskID {
		t.Fatalf("project tasks = %#v, want task %q", project.Tasks, taskID)
	}
}

func TestUpdateProject_PersistsFields(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	project, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	project.Name = "Renamed"
	project.Description = "Updated description"
	project.Completion = 42
	project.Public = false

	updated, err := db.UpdateProject(accountID, project)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if updated.Name != "Renamed" || updated.Description != "Updated description" || updated.Completion != 42 || updated.Public {
		t.Fatalf("updated project = %#v, want renamed private project at 42", updated)
	}

	readBack, err := db.GetProject(accountID, projectID)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	if readBack.Name != "Renamed" || readBack.Description != "Updated description" || readBack.Completion != 42 || readBack.Public {
		t.Fatalf("read project = %#v, want renamed private project at 42", readBack)
	}
}

func TestDeleteProject_RemovesProjectAndDependents(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.DeleteProject(accountID, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	if _, err := db.GetProject(accountID, projectID); err == nil {
		t.Fatal("GetProject succeeded after delete; want error")
	}
	if _, err := db.GetTask(accountID, taskID); err == nil {
		t.Fatal("GetTask succeeded after project delete; want error")
	}
}

func TestReorderProjects_ChangesOnlyCategoryOrdering(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	otherCategoryID := seedCategory(t, db, accountID, "Other Category")
	firstProjectID := seedProject(t, db, accountID, categoryID, "First")
	secondProjectID := seedProject(t, db, accountID, categoryID, "Second")
	otherProjectID := seedProject(t, db, accountID, otherCategoryID, "Other")

	if err := db.ReorderProjects(accountID, categoryID, []string{secondProjectID, firstProjectID}); err != nil {
		t.Fatalf("reorder projects: %v", err)
	}

	category, err := db.GetCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category: %v", err)
	}
	if category.Projects[0].ID != secondProjectID || category.Projects[1].ID != firstProjectID {
		t.Fatalf("project order = [%q, %q], want [%q, %q]", category.Projects[0].ID, category.Projects[1].ID, secondProjectID, firstProjectID)
	}

	otherCategory, err := db.GetCategory(accountID, otherCategoryID)
	if err != nil {
		t.Fatalf("get other category: %v", err)
	}
	if len(otherCategory.Projects) != 1 || otherCategory.Projects[0].ID != otherProjectID {
		t.Fatalf("other projects = %#v, want unchanged project %q", otherCategory.Projects, otherProjectID)
	}
}

func TestGetProject_ScopesByAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	categoryID := seedCategory(t, db, aliceID, "Category")
	projectID := seedProject(t, db, aliceID, categoryID, "Project")

	if _, err := db.GetProject(bobID, projectID); err == nil {
		t.Fatal("GetProject with another account succeeded; want error")
	}
}
