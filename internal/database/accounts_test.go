package database_test

import (
	"errors"
	"testing"

	"git.sr.ht/~jakintosh/compass/internal/service"
)

func TestUpsertAccount_CreatesAndReadsByHandleAndSubject(t *testing.T) {
	db := setupDB(t)

	refreshedAt := fixedTime(0)
	account, err := db.UpsertAccount("subject-alice", "alice", refreshedAt)
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}

	byHandle, err := db.GetAccountByHandle("alice")
	if err != nil {
		t.Fatalf("get account by handle: %v", err)
	}
	if byHandle.ID != account.ID {
		t.Fatalf("handle lookup ID = %q, want %q", byHandle.ID, account.ID)
	}

	bySubject, err := db.GetAccountBySubject("subject-alice")
	if err != nil {
		t.Fatalf("get account by subject: %v", err)
	}
	if bySubject.ID != account.ID {
		t.Fatalf("subject lookup ID = %q, want %q", bySubject.ID, account.ID)
	}
	if !bySubject.ProfileRefreshedAt.Equal(refreshedAt) {
		t.Fatalf("refreshed at = %s, want %s", bySubject.ProfileRefreshedAt, refreshedAt)
	}
}

func TestUpsertAccount_UpdatesExistingSubject(t *testing.T) {
	db := setupDB(t)

	first, err := db.UpsertAccount("subject-alice", "alice", fixedTime(0))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := db.UpsertAccount("subject-alice", "alice-renamed", fixedTime(1))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("account ID changed from %q to %q", first.ID, second.ID)
	}
	if second.Handle != "alice-renamed" {
		t.Fatalf("handle = %q, want alice-renamed", second.Handle)
	}

	byHandle, err := db.GetAccountByHandle("alice-renamed")
	if err != nil {
		t.Fatalf("get account by new handle: %v", err)
	}
	if byHandle.ID != first.ID {
		t.Fatalf("new handle ID = %q, want %q", byHandle.ID, first.ID)
	}
}

func TestGetAccount_MissingAccountReturnsError(t *testing.T) {
	db := setupDB(t)

	if _, err := db.GetAccountByHandle("missing"); err == nil {
		t.Fatal("GetAccountByHandle succeeded for missing handle; want error")
	}
	if _, err := db.GetAccountBySubject("missing"); err == nil {
		t.Fatal("GetAccountBySubject succeeded for missing subject; want error")
	}
}

func TestUpsertAccount_DuplicateHandleForDifferentSubjectFails(t *testing.T) {
	db := setupDB(t)

	if _, err := db.UpsertAccount("subject-alice", "shared", fixedTime(0)); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.UpsertAccount("subject-bob", "shared", fixedTime(1)); !errors.Is(err, service.ErrAccountHandleConflict) {
		t.Fatalf("UpsertAccount with duplicate handle error = %v, want ErrAccountHandleConflict", err)
	}
}
