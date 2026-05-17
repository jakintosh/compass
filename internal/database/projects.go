package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	err := db.Conn.QueryRow(`
		SELECT
			t.id,
			t.category_id,
			t.name,
			t.description,
			t.status,
			t.completion,
			t.public,
			c.public AS parent_public,
			t.created_at,
			t.updated_at,
			t.archived_at,
			t.deleted_at
		FROM projects t
		JOIN categories c ON t.category_id = c.id
		WHERE t.account_id = ?1 AND t.id = ?2 AND t.deleted_at IS NULL`,
		accountID,
		id,
	).Scan(
		&p.ID,
		&p.CategoryID,
		&p.Name,
		&p.Description,
		&p.Status,
		&p.Completion,
		&p.Public,
		&p.ParentPublic,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(createdAt, 0)
	p.UpdatedAt = time.Unix(updatedAt, 0)
	p.ArchivedAt = nullableUnixTime(archivedAt)
	p.DeletedAt = nullableUnixTime(deletedAt)
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
	now := time.Now().Unix()

	var maxOrder sql.NullInt64
	db.Conn.QueryRow(`
		SELECT MAX(sort_order)
		FROM projects
		WHERE account_id = ?1 AND category_id = ?2`,
		accountID,
		catID,
	).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add project tx: %w", err)
	}
	defer tx.Rollback()

	var project service.Project
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := tx.QueryRow(`
		INSERT INTO projects (id, account_id, category_id, name, sort_order, created_at, updated_at)
		SELECT ?1, ?2, id, ?4, ?5, ?6, ?6
		FROM categories
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			category_id,
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
		catID,
		name,
		order,
		now,
	).Scan(
		&project.ID,
		&project.CategoryID,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.Completion,
		&project.Public,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	project.CreatedAt = time.Unix(createdAt, 0)
	project.UpdatedAt = time.Unix(updatedAt, 0)
	project.ArchivedAt = nullableUnixTime(archivedAt)
	project.DeletedAt = nullableUnixTime(deletedAt)
	if err := db.sqlInsertEventTx(tx, accountID, "project", id, "project.created", fmt.Sprintf(`{"category_id":%q}`, catID)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add project tx: %w", err)
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
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := db.Conn.QueryRow(`
		UPDATE projects
		SET name = ?1,
			description = ?2,
			status = ?3,
			completion = ?4,
			public = ?5,
			updated_at = ?6
		WHERE account_id = ?7 AND id = ?8
		RETURNING
			id,
			category_id,
			name,
			description,
			status,
			completion,
			public,
			created_at,
			updated_at,
			archived_at,
			deleted_at`,
		project.Name,
		project.Description,
		project.Status,
		project.Completion,
		project.Public,
		time.Now().Unix(),
		accountID,
		project.ID,
	).Scan(
		&updated.ID,
		&updated.CategoryID,
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
	updated.CreatedAt = time.Unix(createdAt, 0)
	updated.UpdatedAt = time.Unix(updatedAt, 0)
	updated.ArchivedAt = nullableUnixTime(archivedAt)
	updated.DeletedAt = nullableUnixTime(deletedAt)
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
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := db.Conn.QueryRow(`
		DELETE FROM projects
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			category_id,
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
		&removed.CategoryID,
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
			return nil, fmt.Errorf("project not found")
		}
		return nil, err
	}
	removed.CreatedAt = time.Unix(createdAt, 0)
	removed.UpdatedAt = time.Unix(updatedAt, 0)
	removed.ArchivedAt = nullableUnixTime(archivedAt)
	removed.DeletedAt = nullableUnixTime(deletedAt)
	return &removed, nil
}

func (db *DB) MoveProject(
	accountID string,
	projectID string,
	targetCategoryID string,
	targetIndex int,
) (
	*service.Project,
	error,
) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin move project tx: %w", err)
	}
	defer tx.Rollback()

	var sourceCategoryID string
	if err := tx.QueryRow(`
		SELECT category_id
		FROM projects
		WHERE account_id = ?1 AND id = ?2 AND deleted_at IS NULL`,
		accountID,
		projectID,
	).Scan(&sourceCategoryID); err != nil {
		return nil, fmt.Errorf("read project before move: %w", err)
	}

	var targetExists int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM categories
		WHERE account_id = ?1 AND id = ?2 AND deleted_at IS NULL`,
		accountID,
		targetCategoryID,
	).Scan(&targetExists); err != nil {
		return nil, fmt.Errorf("read target category: %w", err)
	}
	if targetExists == 0 {
		return nil, sql.ErrNoRows
	}

	targetIDs, err := db.sqlProjectIDsForCategoryTx(tx, accountID, targetCategoryID, projectID)
	if err != nil {
		return nil, err
	}
	targetIDs = insertIDAt(targetIDs, projectID, targetIndex)

	if _, err := tx.Exec(`
		UPDATE projects
		SET category_id = ?1,
			updated_at = ?2
		WHERE account_id = ?3 AND id = ?4`,
		targetCategoryID,
		time.Now().Unix(),
		accountID,
		projectID,
	); err != nil {
		return nil, fmt.Errorf("move project: %w", err)
	}
	if err := db.sqlReorderProjectsTx(tx, accountID, targetCategoryID, targetIDs); err != nil {
		return nil, err
	}
	if sourceCategoryID != targetCategoryID {
		sourceIDs, err := db.sqlProjectIDsForCategoryTx(tx, accountID, sourceCategoryID, "")
		if err != nil {
			return nil, err
		}
		if err := db.sqlReorderProjectsTx(tx, accountID, sourceCategoryID, sourceIDs); err != nil {
			return nil, err
		}
	}
	if err := db.sqlInsertEventTx(tx, accountID, "project", projectID, "project.moved", fmt.Sprintf(`{"from_category_id":%q,"to_category_id":%q,"to_index":%d}`, sourceCategoryID, targetCategoryID, targetIndex)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit move project tx: %w", err)
	}

	return db.GetProject(accountID, projectID)
}

func (db *DB) ArchiveProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	return db.setProjectStatus(accountID, id, "archived")
}

func (db *DB) RestoreProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	return db.setProjectStatus(accountID, id, "active")
}

func (db *DB) setProjectStatus(
	accountID string,
	id string,
	status string,
) (
	*service.Project,
	error,
) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin set project status tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	var archivedAt any
	if status == "archived" {
		archivedAt = now
	}
	if status == "active" {
		var parentStatus string
		if err := tx.QueryRow(`
			SELECT c.status
			FROM projects p
			JOIN categories c ON p.category_id = c.id
			WHERE p.account_id = ?1 AND p.id = ?2`,
			accountID,
			id,
		).Scan(&parentStatus); err != nil {
			return nil, fmt.Errorf("read project parent status: %w", err)
		}
		if parentStatus != "active" {
			return nil, fmt.Errorf("project parent is not active")
		}
	}

	if _, err := tx.Exec(`
		UPDATE projects
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
		return nil, fmt.Errorf("set project status: %w", err)
	}
	if err := db.sqlInsertEventTx(tx, accountID, "project", id, "project.status_changed", fmt.Sprintf(`{"to_status":%q}`, status)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit set project status tx: %w", err)
	}

	return db.GetProject(accountID, id)
}

func (db *DB) ReorderProjects(
	accountID string,
	catID string,
	projectIDs []string,
) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.sqlReorderProjectsTx(tx, accountID, catID, projectIDs); err != nil {
		return err
	}
	if err := db.sqlInsertEventTx(tx, accountID, "category", catID, "project.reordered", "{}"); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) sqlProjectIDsForCategoryTx(
	tx *sql.Tx,
	accountID string,
	catID string,
	excludeProjectID string,
) (
	[]string,
	error,
) {
	rows, err := tx.Query(`
		SELECT id
		FROM projects
		WHERE account_id = ?1 AND category_id = ?2 AND id != ?3 AND deleted_at IS NULL
		ORDER BY sort_order ASC`,
		accountID,
		catID,
		excludeProjectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project ids for category: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project ids: %w", err)
	}
	return ids, nil
}

func (db *DB) sqlReorderProjectsTx(
	tx *sql.Tx,
	accountID string,
	catID string,
	projectIDs []string,
) error {
	for i, id := range projectIDs {
		if _, err := tx.Exec(`
			UPDATE projects
			SET sort_order = ?1,
				updated_at = ?2
			WHERE account_id = ?3 AND id = ?4 AND category_id = ?5`,
			i,
			time.Now().Unix(),
			accountID,
			id,
			catID,
		); err != nil {
			return fmt.Errorf("reorder project %q: %w", id, err)
		}
	}
	return nil
}

func scanProjectRows(
	rows *sql.Rows,
) (
	*service.Project,
	error,
) {
	var project service.Project
	var createdAt int64
	var updatedAt int64
	var archivedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := rows.Scan(
		&project.ID,
		&project.CategoryID,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.Completion,
		&project.Public,
		&project.ParentPublic,
		&createdAt,
		&updatedAt,
		&archivedAt,
		&deletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan project row: %w", err)
	}
	project.CreatedAt = time.Unix(createdAt, 0)
	project.UpdatedAt = time.Unix(updatedAt, 0)
	project.ArchivedAt = nullableUnixTime(archivedAt)
	project.DeletedAt = nullableUnixTime(deletedAt)
	return &project, nil
}

func insertIDAt(
	ids []string,
	id string,
	index int,
) []string {
	if index < 0 {
		index = 0
	}
	if index > len(ids) {
		index = len(ids)
	}
	ids = append(ids, "")
	copy(ids[index+1:], ids[index:])
	ids[index] = id
	return ids
}
