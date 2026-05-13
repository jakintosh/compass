package database

import (
	"database/sql"
	"errors"
	"fmt"

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
	err := db.Conn.QueryRow(
		`SELECT
			s.id,
			s.project_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM tasks s
		JOIN projects t ON s.project_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1 AND s.id = ?2`,
		accountID,
		id,
	).Scan(
		&sub.ID,
		&sub.ProjectID,
		&sub.CategoryID,
		&sub.Name,
		&sub.Description,
		&sub.Completion,
		&sub.Public,
		&sub.ParentPublic,
	)
	if err != nil {
		return nil, err
	}
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
	if err := tx.QueryRow(`
		INSERT INTO tasks (id, account_id, project_id, category_id, name, sort_order)
		SELECT ?1, ?2, id, category_id, ?4, ?5
		FROM projects
		WHERE account_id = ?2 AND id = ?3
		RETURNING
			id,
			project_id,
			category_id,
			name,
			description,
			completion,
			public`,
		id,
		accountID,
		projectID,
		name,
		order,
	).Scan(
		&sub.ID,
		&sub.ProjectID,
		&sub.CategoryID,
		&sub.Name,
		&sub.Description,
		&sub.Completion,
		&sub.Public,
	); err != nil {
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
	if err := db.Conn.QueryRow(`
		UPDATE tasks
		SET name = ?1,
			description = ?2,
			completion = ?3,
			public = ?4
		WHERE account_id = ?5 AND id = ?6
		RETURNING
			id,
			project_id,
			category_id,
			name,
			description,
			completion,
			public`,
		sub.Name,
		sub.Description,
		sub.Completion,
		sub.Public,
		accountID,
		sub.ID,
	).Scan(
		&updated.ID,
		&updated.ProjectID,
		&updated.CategoryID,
		&updated.Name,
		&updated.Description,
		&updated.Completion,
		&updated.Public,
	); err != nil {
		return nil, err
	}

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
	if err := db.Conn.QueryRow(`
		DELETE FROM tasks
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			project_id,
			category_id,
			name,
			description,
			completion`,
		accountID,
		id,
	).Scan(
		&removed.ID,
		&removed.ProjectID,
		&removed.CategoryID,
		&removed.Name,
		&removed.Description,
		&removed.Completion,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}

	return &removed, nil
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

	for i, id := range taskIDs {
		if _, err := tx.Exec(`
			UPDATE tasks
			SET sort_order = ?1
			WHERE account_id = ?2 AND id = ?3 AND project_id = ?4`,
			i,
			accountID,
			id,
			projectID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanTaskRows(
	rows *sql.Rows,
) (
	*service.Task,
	error,
) {
	var task service.Task
	if err := rows.Scan(
		&task.ID,
		&task.ProjectID,
		&task.CategoryID,
		&task.Name,
		&task.Description,
		&task.Completion,
		&task.Public,
		&task.ParentPublic,
	); err != nil {
		return nil, fmt.Errorf("scan task row: %w", err)
	}
	return &task, nil
}
