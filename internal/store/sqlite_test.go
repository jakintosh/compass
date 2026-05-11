package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreScopesDataByAccount(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "compass.db"), false)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}

	alice, err := store.UpsertAccount("subject-alice", "alice", time.Now())
	if err != nil {
		t.Fatalf("UpsertAccount(alice) returned error: %v", err)
	}
	bob, err := store.UpsertAccount("subject-bob", "bob", time.Now())
	if err != nil {
		t.Fatalf("UpsertAccount(bob) returned error: %v", err)
	}

	aliceCat, err := store.AddCategory(alice.ID, "Alice Category")
	if err != nil {
		t.Fatalf("AddCategory(alice) returned error: %v", err)
	}
	if _, err := store.AddCategory(bob.ID, "Bob Category"); err != nil {
		t.Fatalf("AddCategory(bob) returned error: %v", err)
	}

	aliceCats, err := store.GetCategories(alice.ID)
	if err != nil {
		t.Fatalf("GetCategories(alice) returned error: %v", err)
	}
	if len(aliceCats) != 1 || aliceCats[0].Name != "Alice Category" {
		t.Fatalf("alice categories = %#v, want only Alice Category", aliceCats)
	}

	bobCats, err := store.GetCategories(bob.ID)
	if err != nil {
		t.Fatalf("GetCategories(bob) returned error: %v", err)
	}
	if len(bobCats) != 1 || bobCats[0].Name != "Bob Category" {
		t.Fatalf("bob categories = %#v, want only Bob Category", bobCats)
	}

	if _, err := store.GetCategory(bob.ID, aliceCat.ID); err == nil {
		t.Fatal("GetCategory(bob, aliceCategoryID) succeeded; want account-scoped miss")
	}
}
