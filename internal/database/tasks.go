package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
)

func (db *DB) GetTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	var sub service.Task
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	err := db.Conn.QueryRow(
		`SELECT
			s.id,
			s.project_id,
			t.category_id,
			s.name,
			s.description,
			s.status,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public,
			s.created_at,
			s.updated_at,
			s.archived_at,
			s.deleted_at
		FROM tasks s
		JOIN projects t ON s.project_id = t.id
		JOIN categories c ON t.category_id = c.id
		WHERE s.account_id = ?1 AND s.id = ?2 AND s.deleted_at IS NULL`,
		accountID,
		id,
	).Scan(
		&sub.ID,
		&sub.ProjectID,
		&sub.CategoryID,
		&sub.Name,
		&sub.Description,
		&sub.Status,
		&sub.Completion,
		&sub.Public,
		&sub.ParentPublic,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	sub.CreatedAt = time.Unix(createdAt, 0)
	sub.UpdatedAt = time.Unix(updatedAt, 0)
	sub.ArchivedAt = nullableUnixTime(archivedAt)
	sub.DeletedAt = nullableUnixTime(deletedAt)
	return &sub, nil
}

func (db *DB) AddTask(
	accountID string,
	projectID string,
	name string,
) (
	*service.Task,
	error,
) {
	id := uuid.NewString()
	now := time.Now().Unix()

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var maxOrder sql.NullInt64
	if err := tx.QueryRow(`
		SELECT MAX(sort_order)
		FROM tasks
		WHERE account_id = ?1 AND project_id = ?2`,
		accountID,
		projectID,
	).Scan(&maxOrder); err != nil {
		return nil, err
	}
	order := int(maxOrder.Int64) + 1

	var sub service.Task
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := tx.QueryRow(`
		INSERT INTO tasks (id, account_id, project_id, name, sort_order, created_at, updated_at)
		SELECT ?1, ?2, id, ?4, ?5, ?6, ?6
		FROM projects
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			project_id,
			name,
			description,
			status,
			completion,
			public,
			created_at,
			updated_at,
			archived_at,
			deleted_at`,
		id,
		accountID,
		projectID,
		name,
		order,
		now,
	).Scan(
		&sub.ID,
		&sub.ProjectID,
		&sub.Name,
		&sub.Description,
		&sub.Status,
		&sub.Completion,
		&sub.Public,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(`
		SELECT category_id
		FROM projects
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		projectID,
	).Scan(&sub.CategoryID); err != nil {
		return nil, err
	}
	sub.CreatedAt = time.Unix(createdAt, 0)
	sub.UpdatedAt = time.Unix(updatedAt, 0)
	sub.ArchivedAt = nullableUnixTime(archivedAt)
	sub.DeletedAt = nullableUnixTime(deletedAt)
	if err := db.sqlInsertEventTx(tx, accountID, "task", id, "task.created", fmt.Sprintf(`{"project_id":%q}`, projectID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &sub, nil
}

func (db *DB) UpdateTask(
	accountID string,
	sub *service.Task,
) (
	*service.Task,
	error,
) {
	var updated service.Task
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := db.Conn.QueryRow(`
		UPDATE tasks
		SET name = ?1,
			description = ?2,
			status = ?3,
			completion = ?4,
			public = ?5,
			updated_at = ?6
		WHERE account_id = ?7 AND id = ?8
		RETURNING
			id,
			project_id,
			name,
			description,
			status,
			completion,
			public,
			created_at,
			updated_at,
			archived_at,
			deleted_at`,
		sub.Name,
		sub.Description,
		sub.Status,
		sub.Completion,
		sub.Public,
		time.Now().Unix(),
		accountID,
		sub.ID,
	).Scan(
		&updated.ID,
		&updated.ProjectID,
		&updated.Name,
		&updated.Description,
		&updated.Status,
		&updated.Completion,
		&updated.Public,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if err := db.Conn.QueryRow(`
		SELECT category_id
		FROM projects
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		updated.ProjectID,
	).Scan(&updated.CategoryID); err != nil {
		return nil, err
	}
	updated.CreatedAt = time.Unix(createdAt, 0)
	updated.UpdatedAt = time.Unix(updatedAt, 0)
	updated.ArchivedAt = nullableUnixTime(archivedAt)
	updated.DeletedAt = nullableUnixTime(deletedAt)

	return &updated, nil
}

func (db *DB) DeleteTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	var removed service.Task
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	var projectID string
	var categoryID string
	if err := db.Conn.QueryRow(`
		SELECT p.category_id
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.account_id = ?1 AND t.id = ?2`,
		accountID,
		id,
	).Scan(&categoryID); err != nil {
		return nil, err
	}
	if err := db.Conn.QueryRow(`
		DELETE FROM tasks
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			project_id,
			name,
			description,
			status,
			completion,
			public,
			created_at,
			updated_at,
			archived_at,
			deleted_at`,
		accountID,
		id,
	).Scan(
		&removed.ID,
		&projectID,
		&removed.Name,
		&removed.Description,
		&removed.Status,
		&removed.Completion,
		&removed.Public,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}
	removed.ProjectID = projectID
	removed.CategoryID = categoryID
	removed.CreatedAt = time.Unix(createdAt, 0)
	removed.UpdatedAt = time.Unix(updatedAt, 0)
	removed.ArchivedAt = nullableUnixTime(archivedAt)
	removed.DeletedAt = nullableUnixTime(deletedAt)

	return &removed, nil
}

func (db *DB) MoveTask(
	accountID string,
	taskID string,
	targetProjectID string,
	targetIndex int,
) (
	*service.Task,
	error,
) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin move task tx: %w", err)
	}
	defer tx.Rollback()

	var sourceProjectID string
	if err := tx.QueryRow(`
		SELECT project_id
		FROM tasks
		WHERE account_id = ?1 AND id = ?2 AND deleted_at IS NULL`,
		accountID,
		taskID,
	).Scan(&sourceProjectID); err != nil {
		return nil, fmt.Errorf("read task before move: %w", err)
	}

	var targetExists int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM projects
		WHERE account_id = ?1 AND id = ?2 AND deleted_at IS NULL`,
		accountID,
		targetProjectID,
	).Scan(&targetExists); err != nil {
		return nil, fmt.Errorf("read target project: %w", err)
	}
	if targetExists == 0 {
		return nil, sql.ErrNoRows
	}

	targetIDs, err := db.sqlTaskIDsForProjectTx(tx, accountID, targetProjectID, taskID)
	if err != nil {
		return nil, err
	}
	targetIDs = insertIDAt(targetIDs, taskID, targetIndex)

	if _, err := tx.Exec(`
		UPDATE tasks
		SET project_id = ?1,
			updated_at = ?2
		WHERE account_id = ?3 AND id = ?4`,
		targetProjectID,
		time.Now().Unix(),
		accountID,
		taskID,
	); err != nil {
		return nil, fmt.Errorf("move task: %w", err)
	}
	if err := db.sqlReorderTasksTx(tx, accountID, targetProjectID, targetIDs); err != nil {
		return nil, err
	}
	if sourceProjectID != targetProjectID {
		sourceIDs, err := db.sqlTaskIDsForProjectTx(tx, accountID, sourceProjectID, "")
		if err != nil {
			return nil, err
		}
		if err := db.sqlReorderTasksTx(tx, accountID, sourceProjectID, sourceIDs); err != nil {
			return nil, err
		}
	}
	if err := db.sqlInsertEventTx(tx, accountID, "task", taskID, "task.moved", fmt.Sprintf(`{"from_project_id":%q,"to_project_id":%q,"to_index":%d}`, sourceProjectID, targetProjectID, targetIndex)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit move task tx: %w", err)
	}

	return db.GetTask(accountID, taskID)
}

func (db *DB) ArchiveTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	return db.setTaskStatus(accountID, id, "archived")
}

func (db *DB) RestoreTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	return db.setTaskStatus(accountID, id, "active")
}

func (db *DB) setTaskStatus(
	accountID string,
	id string,
	status string,
) (
	*service.Task,
	error,
) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin set task status tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	var archivedAt any
	if status == "archived" {
		archivedAt = now
	}
	if status == "active" {
		var projectStatus string
		var categoryStatus string
		if err := tx.QueryRow(`
			SELECT p.status, c.status
			FROM tasks t
			JOIN projects p ON t.project_id = p.id
			JOIN categories c ON p.category_id = c.id
			WHERE t.account_id = ?1 AND t.id = ?2`,
			accountID,
			id,
		).Scan(&projectStatus, &categoryStatus); err != nil {
			return nil, fmt.Errorf("read task parent status: %w", err)
		}
		if projectStatus != "active" || categoryStatus != "active" {
			return nil, fmt.Errorf("task parent is not active")
		}
	}

	if _, err := tx.Exec(`
		UPDATE tasks
		SET status = ?1,
			updated_at = ?2,
			archived_at = ?3
		WHERE account_id = ?4 AND id = ?5 AND deleted_at IS NULL`,
		status,
		now,
		archivedAt,
		accountID,
		id,
	); err != nil {
		return nil, fmt.Errorf("set task status: %w", err)
	}
	if err := db.sqlInsertEventTx(tx, accountID, "task", id, "task.status_changed", fmt.Sprintf(`{"to_status":%q}`, status)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit set task status tx: %w", err)
	}

	return db.GetTask(accountID, id)
}

func (db *DB) ReorderTasks(
	accountID string,
	projectID string,
	taskIDs []string,
) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.sqlReorderTasksTx(tx, accountID, projectID, taskIDs); err != nil {
		return err
	}
	if err := db.sqlInsertEventTx(tx, accountID, "project", projectID, "task.reordered", "{}"); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) sqlTaskIDsForProjectTx(
	tx *sql.Tx,
	accountID string,
	projectID string,
	excludeTaskID string,
) (
	[]string,
	error,
) {
	rows, err := tx.Query(`
		SELECT id
		FROM tasks
		WHERE account_id = ?1 AND project_id = ?2 AND id != ?3 AND deleted_at IS NULL
		ORDER BY sort_order ASC`,
		accountID,
		projectID,
		excludeTaskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task ids for project: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan task id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task ids: %w", err)
	}
	return ids, nil
}

func (db *DB) sqlReorderTasksTx(
	tx *sql.Tx,
	accountID string,
	projectID string,
	taskIDs []string,
) error {
	for i, id := range taskIDs {
		if _, err := tx.Exec(`
			UPDATE tasks
			SET sort_order = ?1,
				updated_at = ?2
			WHERE account_id = ?3 AND id = ?4 AND project_id = ?5`,
			i,
			time.Now().Unix(),
			accountID,
			id,
			projectID,
		); err != nil {
			return fmt.Errorf("reorder task %q: %w", id, err)
		}
	}
	return nil
}

func scanTaskRows(
	rows *sql.Rows,
) (
	*service.Task,
	error,
) {
	var task service.Task
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := rows.Scan(
		&task.ID,
		&task.ProjectID,
		&task.CategoryID,
		&task.Name,
		&task.Description,
		&task.Status,
		&task.Completion,
		&task.Public,
		&task.ParentPublic,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan task row: %w", err)
	}
	task.CreatedAt = time.Unix(createdAt, 0)
	task.UpdatedAt = time.Unix(updatedAt, 0)
	task.ArchivedAt = nullableUnixTime(archivedAt)
	task.DeletedAt = nullableUnixTime(deletedAt)
	return &task, nil
}
