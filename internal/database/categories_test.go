package database_test

import "testing"

func TestAddAndGetCategory_ScopesByAccount(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")

	aliceCategoryID := seedCategory(t, db, aliceID, "Alice Category")
	seedCategory(t, db, bobID, "Bob Category")

	aliceCategory, err := db.GetCategory(aliceID, aliceCategoryID)
	if err != nil {
		t.Fatalf("get alice category: %v", err)
	}
	if aliceCategory.Name != "Alice Category" {
		t.Fatalf("category name = %q, want Alice Category", aliceCategory.Name)
	}

	if _, err := db.GetCategory(bobID, aliceCategoryID); err == nil {
		t.Fatal("GetCategory with another account succeeded; want error")
	}
}

func TestGetCategories_ReturnsOrderedNestedTree(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")

	firstCategoryID := seedCategory(t, db, accountID, "First Category")
	secondCategoryID := seedCategory(t, db, accountID, "Second Category")
	projectID := seedProject(t, db, accountID, secondCategoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	categories, err := db.GetCategories(accountID)
	if err != nil {
		t.Fatalf("get categories: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("category count = %d, want 2", len(categories))
	}
	if categories[0].ID != secondCategoryID || categories[1].ID != firstCategoryID {
		t.Fatalf("category order = [%q, %q], want [%q, %q]", categories[0].ID, categories[1].ID, secondCategoryID, firstCategoryID)
	}
	if len(categories[0].Projects) != 1 {
		t.Fatalf("project count = %d, want 1", len(categories[0].Projects))
	}
	if categories[0].Projects[0].ID != projectID {
		t.Fatalf("project ID = %q, want %q", categories[0].Projects[0].ID, projectID)
	}
	if len(categories[0].Projects[0].Tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(categories[0].Projects[0].Tasks))
	}
	if categories[0].Projects[0].Tasks[0].ID != taskID {
		t.Fatalf("task ID = %q, want %q", categories[0].Projects[0].Tasks[0].ID, taskID)
	}
}

func TestUpdateCategory_PersistsFields(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")

	category, err := db.GetCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("get category: %v", err)
	}
	category.Name = "Renamed"
	category.Description = "Updated description"
	category.Public = false

	updated, err := db.UpdateCategory(accountID, category)
	if err != nil {
		t.Fatalf("update category: %v", err)
	}
	if updated.Name != "Renamed" || updated.Description != "Updated description" || updated.Public {
		t.Fatalf("updated category = %#v, want renamed private category", updated)
	}

	readBack, err := db.GetCategory(accountID, categoryID)
	if err != nil {
		t.Fatalf("read updated category: %v", err)
	}
	if readBack.Name != "Renamed" || readBack.Description != "Updated description" || readBack.Public {
		t.Fatalf("read category = %#v, want renamed private category", readBack)
	}
}

func TestDeleteCategory_RemovesCategoryAndDependents(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")
	taskID := seedTask(t, db, accountID, projectID, "Task")

	if _, err := db.DeleteCategory(accountID, categoryID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	if _, err := db.GetCategory(accountID, categoryID); err == nil {
		t.Fatal("GetCategory succeeded after delete; want error")
	}
	if _, err := db.GetProject(accountID, projectID); err == nil {
		t.Fatal("GetProject succeeded after category delete; want error")
	}
	if _, err := db.GetTask(accountID, taskID); err == nil {
		t.Fatal("GetTask succeeded after category delete; want error")
	}
}

func TestReorderCategories_ChangesOnlyAccountOrdering(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")

	aliceFirstID := seedCategory(t, db, aliceID, "Alice First")
	aliceSecondID := seedCategory(t, db, aliceID, "Alice Second")
	bobFirstID := seedCategory(t, db, bobID, "Bob First")

	if err := db.ReorderCategories(aliceID, []string{aliceFirstID, aliceSecondID}); err != nil {
		t.Fatalf("reorder categories: %v", err)
	}

	aliceCategories, err := db.GetCategories(aliceID)
	if err != nil {
		t.Fatalf("get alice categories: %v", err)
	}
	if aliceCategories[0].ID != aliceFirstID || aliceCategories[1].ID != aliceSecondID {
		t.Fatalf("alice order = [%q, %q], want [%q, %q]", aliceCategories[0].ID, aliceCategories[1].ID, aliceFirstID, aliceSecondID)
	}

	bobCategories, err := db.GetCategories(bobID)
	if err != nil {
		t.Fatalf("get bob categories: %v", err)
	}
	if len(bobCategories) != 1 || bobCategories[0].ID != bobFirstID {
		t.Fatalf("bob categories = %#v, want unchanged category %q", bobCategories, bobFirstID)
	}
}
