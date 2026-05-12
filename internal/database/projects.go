package database

import (
	"database/sql"
	"errors"
	"fmt"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
)

func (db *DB) GetProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	var p service.Project
	err := db.Conn.QueryRow(`
		SELECT
			t.id,
			t.category_id,
			t.name,
			t.description,
			t.completion,
			t.public,
			c.public AS parent_public
		FROM tasks t
		JOIN categories c ON t.category_id = c.id
		WHERE t.account_id = ?1 AND t.id = ?2`,
		accountID,
		id,
	).Scan(
		&p.ID,
		&p.CategoryID,
		&p.Name,
		&p.Description,
		&p.Completion,
		&p.Public,
		&p.ParentPublic,
	)
	if err != nil {
		return nil, err
	}
	tasks, err := db.getTasksForProject(accountID, p.ID)
	if err != nil {
		return nil, err
	}
	p.Tasks = tasks
	return &p, nil
}

func (db *DB) AddProject(
	accountID string,
	catID string,
	name string,
) (
	*service.Project,
	error,
) {
	id := uuid.NewString()

	var maxOrder sql.NullInt64
	db.Conn.QueryRow(`
		SELECT MAX(sort_order)
		FROM tasks
		WHERE account_id = ?1 AND category_id = ?2`,
		accountID,
		catID,
	).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	var project service.Project
	if err := db.Conn.QueryRow(`
		INSERT INTO tasks (id, account_id, category_id, name, sort_order)
		SELECT ?1, ?2, id, ?4, ?5
		FROM categories
		WHERE account_id = ?2 AND id = ?3
		RETURNING
			id,
			category_id,
			name,
			description,
			completion,
			public`,
		id,
		accountID,
		catID,
		name,
		order,
	).Scan(
		&project.ID,
		&project.CategoryID,
		&project.Name,
		&project.Description,
		&project.Completion,
		&project.Public,
	); err != nil {
		return nil, err
	}

	project.Tasks = []*service.Task{}
	return &project, nil
}

func (db *DB) UpdateProject(
	accountID string,
	project *service.Project,
) (
	*service.Project,
	error,
) {
	var updated service.Project
	if err := db.Conn.QueryRow(`
		UPDATE tasks
		SET name = ?1,
			description = ?2,
			completion = ?3,
			public = ?4
		WHERE account_id = ?5 AND id = ?6
		RETURNING
			id,
			category_id,
			name,
			description,
			completion,
			public`,
		project.Name,
		project.Description,
		project.Completion,
		project.Public,
		accountID,
		project.ID,
	).Scan(
		&updated.ID,
		&updated.CategoryID,
		&updated.Name,
		&updated.Description,
		&updated.Completion,
		&updated.Public,
	); err != nil {
		return nil, err
	}
	updated.Tasks = project.Tasks
	return &updated, nil
}

func (db *DB) DeleteProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	var removed service.Project
	if err := db.Conn.QueryRow(`
		DELETE FROM tasks
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			category_id,
			name,
			description,
			completion`,
		accountID,
		id,
	).Scan(
		&removed.ID,
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

func (db *DB) ReorderProjects(
	accountID string,
	catID string,
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
			WHERE account_id = ?2 AND id = ?3 AND category_id = ?4`,
			i,
			accountID,
			id,
			catID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanProjectRows(
	rows *sql.Rows,
) (
	*service.Project,
	error,
) {
	var project service.Project
	if err := rows.Scan(
		&project.ID,
		&project.CategoryID,
		&project.Name,
		&project.Description,
		&project.Completion,
		&project.Public,
		&project.ParentPublic,
	); err != nil {
		return nil, fmt.Errorf("scan project row: %w", err)
	}
	return &project, nil
}
