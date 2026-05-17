package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (db *DB) sqlInsertEventTx(
	tx *sql.Tx,
	accountID string,
	entityType string,
	entityID string,
	eventType string,
	dataJSON string,
) error {
	if dataJSON == "" {
		dataJSON = "{}"
	}

	if _, err := tx.Exec(`
		INSERT INTO entity_events (
			id,
			account_id,
			actor_account_id,
			entity_type,
			entity_id,
			event_type,
			occurred_at,
			data_json
		)
		VALUES (?1, ?2, ?2, ?3, ?4, ?5, ?6, ?7)`,
		uuid.NewString(),
		accountID,
		entityType,
		entityID,
		eventType,
		time.Now().Unix(),
		dataJSON,
	); err != nil {
		return fmt.Errorf("insert entity event: %w", err)
	}

	return nil
}

func nullableUnixTime(
	value sql.NullInt64,
) *time.Time {
	if !value.Valid {
		return nil
	}
	t := time.Unix(value.Int64, 0)
	return &t
}
