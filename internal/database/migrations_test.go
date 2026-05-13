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
		"work_logs",
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
		"idx_tasks_account",
		"idx_work_logs_account",
		"idx_work_logs_category",
		"idx_work_logs_project",
		"idx_work_logs_task",
		"idx_work_logs_created_at",
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
