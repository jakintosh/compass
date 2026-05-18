package database

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (db *DB) GetAccountByHandle(
	handle string,
) (
	*service.Account,
	error,
) {
	return scanAccount(db.Conn.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE handle = ?1`,
		handle,
	))
}

func (db *DB) GetAccountBySubject(
	subject string,
) (
	*service.Account,
	error,
) {
	return scanAccount(db.Conn.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE consent_subject = ?1`,
		subject,
	))
}

func (db *DB) UpsertAccount(
	subject string,
	handle string,
	refreshedAt time.Time,
) (
	*service.Account,
	error,
) {
	id := uuid.NewString()
	now := time.Now().Unix()
	refreshed := refreshedAt.Unix()
	account, err := scanAccount(db.Conn.QueryRow(`
		INSERT INTO accounts (id, consent_subject, handle, profile_refreshed_at, created_at, updated_at)
		VALUES (?1, ?2, ?3, ?4, ?5, ?5)
		ON CONFLICT(consent_subject) DO UPDATE SET
			handle = excluded.handle,
			profile_refreshed_at = excluded.profile_refreshed_at,
			updated_at = ?5
		RETURNING id, consent_subject, handle, profile_refreshed_at, created_at, updated_at`,
		id,
		subject,
		handle,
		refreshed,
		now,
	))
	if isAccountHandleConflict(err) {
		return nil, service.ErrAccountHandleConflict
	}
	return account, err
}

func isAccountHandleConflict(
	err error,
) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "accounts.handle")
}

func scanAccount(
	row *sql.Row,
) (
	*service.Account,
	error,
) {
	var account service.Account
	var profileRefreshedAt int64
	var createdAt int64
	var updatedAt int64
	if err := row.Scan(
		&account.ID,
		&account.ConsentSubject,
		&account.Handle,
		&profileRefreshedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	account.ProfileRefreshedAt = time.Unix(profileRefreshedAt, 0)
	account.CreatedAt = time.Unix(createdAt, 0)
	account.UpdatedAt = time.Unix(updatedAt, 0)
	return &account, nil
}
