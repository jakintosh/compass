package database_test

import (
	"path/filepath"
	"testing"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/database"
)

func setupDB(t *testing.T) *database.DB {
	t.Helper()

	opts := database.Options{
		Path: ":memory:",
		WAL:  false,
	}

	db, err := database.Open(opts)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func setupFileDB(t *testing.T) (string, *database.DB) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "compass.db")
	opts := database.Options{
		Path: path,
		WAL:  false,
	}

	db, err := database.Open(opts)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return path, db
}

func seedAccount(t *testing.T, db *database.DB, label string) string {
	t.Helper()

	account, err := db.UpsertAccount("subject-"+label, label, fixedTime(0))
	if err != nil {
		t.Fatalf("seed account %q: %v", label, err)
	}
	return account.ID
}

func seedCategory(t *testing.T, db *database.DB, accountID string, name string) string {
	t.Helper()

	category, err := db.AddCategory(accountID, name)
	if err != nil {
		t.Fatalf("seed category %q: %v", name, err)
	}
	return category.ID
}

func seedProject(t *testing.T, db *database.DB, accountID string, categoryID string, name string) string {
	t.Helper()

	project, err := db.AddProject(accountID, categoryID, name)
	if err != nil {
		t.Fatalf("seed project %q: %v", name, err)
	}
	return project.ID
}

func seedTask(t *testing.T, db *database.DB, accountID string, projectID string, name string) string {
	t.Helper()

	task, err := db.AddTask(accountID, projectID, name)
	if err != nil {
		t.Fatalf("seed task %q: %v", name, err)
	}
	return task.ID
}

func fixedTime(offsetHours int) time.Time {
	return time.Date(2026, 1, 15, 12+offsetHours, 0, 0, 0, time.UTC)
}

func TestOpen_ReturnsUsableDatabase(t *testing.T) {
	db := setupDB(t)

	accountID := seedAccount(t, db, "alice")
	got, err := db.GetAccountByHandle("alice")
	if err != nil {
		t.Fatalf("get seeded account: %v", err)
	}
	if got.ID != accountID {
		t.Fatalf("account ID = %q, want %q", got.ID, accountID)
	}
}

func TestClose_Succeeds(t *testing.T) {
	opts := database.Options{
		Path: ":memory:",
		WAL:  false,
	}

	db, err := database.Open(opts)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
}

func TestOpen_FileBackedDatabasePreservesDataAfterReopen(t *testing.T) {
	path, db := setupFileDB(t)

	accountID := seedAccount(t, db, "alice")
	if err := db.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
	}

	opts := database.Options{
		Path: path,
		WAL:  false,
	}
	reopened, err := database.Open(opts)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() {
		_ = reopened.Close()
	})

	account, err := reopened.GetAccountByHandle("alice")
	if err != nil {
		t.Fatalf("get account after reopen: %v", err)
	}
	if account.ID != accountID {
		t.Fatalf("account ID = %q, want %q", account.ID, accountID)
	}
}

func TestOpen_FileBackedDatabaseSupportsWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compass.db")
	opts := database.Options{
		Path: path,
		WAL:  true,
	}

	db, err := database.Open(opts)
	if err != nil {
		t.Fatalf("open wal database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var mode string
	if err := db.Conn.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}
