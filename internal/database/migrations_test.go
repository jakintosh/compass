package database_test

import "testing"

func TestMigrations_SetSchemaVersion(t *testing.T) {
	db := setupDB(t)

	var version int
	if err := db.Conn.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
	}
}

func TestMigrations_CreateRequiredTables(t *testing.T) {
	db := setupDB(t)

	for _, table := range []string{
		"accounts",
		"categories",
		"projects",
		"tasks",
		"task_log",
		"project_log",
		"entity_events",
	} {
		var count int
		if err := db.Conn.QueryRow(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type = 'table' AND name = ?1`,
			table,
		).Scan(&count); err != nil {
			t.Fatalf("query table %q: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %q count = %d, want 1", table, count)
		}
	}
}

func TestMigrations_CreateRequiredIndexes(t *testing.T) {
	db := setupDB(t)

	for _, index := range []string{
		"idx_categories_account",
		"idx_projects_account",
		"idx_projects_category",
		"idx_tasks_account",
		"idx_tasks_project",
		"idx_task_log_account",
		"idx_task_log_task",
		"idx_task_log_created_at",
		"idx_project_log_account",
		"idx_project_log_project",
		"idx_project_log_created_at",
		"idx_entity_events_account",
		"idx_entity_events_entity",
		"idx_entity_events_occurred_at",
	} {
		var count int
		if err := db.Conn.QueryRow(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type = 'index' AND name = ?1`,
			index,
		).Scan(&count); err != nil {
			t.Fatalf("query index %q: %v", index, err)
		}
		if count != 1 {
			t.Fatalf("index %q count = %d, want 1", index, count)
		}
	}
}

func TestMigrations_EnableForeignKeys(t *testing.T) {
	db := setupDB(t)

	_, err := db.Conn.Exec(`
		INSERT INTO categories (id, account_id, name)
		VALUES ('category-1', 'missing-account', 'Category')`,
	)
	if err == nil {
		t.Fatal("insert category with missing account succeeded; want foreign key error")
	}
}

func TestMigrations_RejectInvalidDomainValues(t *testing.T) {
	db := setupDB(t)
	accountID := seedAccount(t, db, "alice")
	categoryID := seedCategory(t, db, accountID, "Category")
	projectID := seedProject(t, db, accountID, categoryID, "Project")

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "invalid category status",
			sql: `
				UPDATE categories
				SET status = 'paused'
				WHERE account_id = ?1 AND id = ?2`,
			args: []any{accountID, categoryID},
		},
		{
			name: "invalid project completion",
			sql: `
				UPDATE projects
				SET completion = 101
				WHERE account_id = ?1 AND id = ?2`,
			args: []any{accountID, projectID},
		},
		{
			name: "invalid public flag",
			sql: `
				UPDATE projects
				SET public = 2
				WHERE account_id = ?1 AND id = ?2`,
			args: []any{accountID, projectID},
		},
		{
			name: "negative task work hours",
			sql: `
				INSERT INTO task_log (
					id,
					account_id,
					task_id,
					hours_worked,
					work_description,
					completion_estimate,
					created_at
				)
				VALUES ('log-invalid-hours', ?1, ?2, -1, '', 10, 1)`,
			args: []any{accountID, seedTask(t, db, accountID, projectID, "Task")},
		},
		{
			name: "invalid project log confidence",
			sql: `
				INSERT INTO project_log (
					id,
					account_id,
					project_id,
					status_estimate,
					confidence,
					created_at
				)
				VALUES ('log-invalid-confidence', ?1, ?2, 10, 'unknown', 1)`,
			args: []any{accountID, projectID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Conn.Exec(tt.sql, tt.args...); err == nil {
				t.Fatal("statement succeeded; want constraint error")
			}
		})
	}
}

func TestMigrations_CompositeForeignKeysPreventAccountDrift(t *testing.T) {
	db := setupDB(t)
	aliceID := seedAccount(t, db, "alice")
	bobID := seedAccount(t, db, "bob")
	aliceCategoryID := seedCategory(t, db, aliceID, "Alice Category")
	aliceProjectID := seedProject(t, db, aliceID, aliceCategoryID, "Alice Project")
	bobCategoryID := seedCategory(t, db, bobID, "Bob Category")
	bobProjectID := seedProject(t, db, bobID, bobCategoryID, "Bob Project")
	bobTaskID := seedTask(t, db, bobID, bobProjectID, "Bob Task")

	if _, err := db.Conn.Exec(`
		INSERT INTO projects (
			id,
			account_id,
			category_id,
			name,
			created_at,
			updated_at
		)
		VALUES ('cross-account-project', ?1, ?2, 'Project', 1, 1)`,
		aliceID,
		bobCategoryID,
	); err == nil {
		t.Fatal("cross-account project insert succeeded; want foreign key error")
	}

	if _, err := db.Conn.Exec(`
		INSERT INTO tasks (
			id,
			account_id,
			project_id,
			name,
			created_at,
			updated_at
		)
		VALUES ('cross-account-task', ?1, ?2, 'Task', 1, 1)`,
		aliceID,
		bobProjectID,
	); err == nil {
		t.Fatal("cross-account task insert succeeded; want foreign key error")
	}

	if _, err := db.Conn.Exec(`
		INSERT INTO project_log (
			id,
			account_id,
			project_id,
			status_estimate,
			confidence,
			created_at
		)
		VALUES ('cross-account-project-log', ?1, ?2, 10, 'medium', 1)`,
		aliceID,
		bobProjectID,
	); err == nil {
		t.Fatal("cross-account project log insert succeeded; want foreign key error")
	}

	if _, err := db.Conn.Exec(`
		INSERT INTO task_log (
			id,
			account_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		)
		VALUES ('cross-account-task-log', ?1, ?2, 1, '', 10, 1)`,
		aliceID,
		bobTaskID,
	); err == nil {
		t.Fatal("cross-account task log insert succeeded; want foreign key error")
	}

	if _, err := db.Conn.Exec(`
		INSERT INTO entity_events (
			id,
			account_id,
			actor_account_id,
			entity_type,
			entity_id,
			event_type,
			occurred_at
		)
		VALUES ('cross-account-event', ?1, ?2, 'project', ?3, 'project.moved', 1)`,
		aliceID,
		bobID,
		aliceProjectID,
	); err == nil {
		t.Fatal("cross-account actor event insert succeeded; want foreign key error")
	}
}
