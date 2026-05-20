package database

import (
	"database/sql"
	"fmt"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
)

func (db *DB) AddTaskLog(
	accountID string,
	taskID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	customTime *time.Time,
) (
	*service.TaskLog,
	error,
) {
	id := uuid.NewString()
	createdAt := logCreatedAt(customTime)

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add task log tx: %w", err)
	}
	defer tx.Rollback()

	taskLog, err := db.sqlInsertTaskLogTx(
		tx,
		id,
		accountID,
		taskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		createdAt,
	)
	if err != nil {
		return nil, err
	}
	if err := db.sqlUpdateTaskCompletionTx(tx, accountID, taskID, completionEstimate); err != nil {
		return nil, err
	}
	if err := db.sqlInsertEventTx(tx, accountID, "task_log", id, "task_log.created", fmt.Sprintf(`{"task_id":%q}`, taskID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add task log tx: %w", err)
	}

	return taskLog, nil
}

func (db *DB) AddProjectLog(
	accountID string,
	projectID string,
	statusEstimate int,
	confidence string,
	note string,
	customTime *time.Time,
) (
	*service.ProjectLog,
	error,
) {
	id := uuid.NewString()
	createdAt := logCreatedAt(customTime)

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add project log tx: %w", err)
	}
	defer tx.Rollback()

	projectLog, err := db.sqlInsertProjectLogTx(
		tx,
		id,
		accountID,
		projectID,
		statusEstimate,
		confidence,
		note,
		createdAt,
	)
	if err != nil {
		return nil, err
	}
	if err := db.sqlUpdateProjectStatusTx(tx, accountID, projectID, statusEstimate, confidence); err != nil {
		return nil, err
	}
	if err := db.sqlInsertEventTx(tx, accountID, "project_log", id, "project_log.created", fmt.Sprintf(`{"project_id":%q}`, projectID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add project log tx: %w", err)
	}

	return projectLog, nil
}

func (db *DB) GetTaskLogsForTask(
	accountID string,
	taskID string,
) (
	[]*service.TaskLog,
	error,
) {
	rows, err := db.Conn.Query(taskLogSelectSQL()+`
		WHERE tl.account_id = ?1
			AND tl.task_id = ?2
			AND tl.deleted_at IS NULL
		ORDER BY tl.created_at DESC`,
		accountID,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task logs for task: %w", err)
	}
	defer rows.Close()
	return scanTaskLogRows(rows)
}

func (db *DB) GetTaskLogsForProject(
	accountID string,
	projectID string,
) (
	[]*service.TaskLog,
	error,
) {
	rows, err := db.Conn.Query(taskLogSelectSQL()+`
		WHERE tl.account_id = ?1
			AND task.project_id = ?2
			AND tl.deleted_at IS NULL
		ORDER BY tl.created_at DESC`,
		accountID,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task logs for project: %w", err)
	}
	defer rows.Close()
	return scanTaskLogRows(rows)
}

func (db *DB) GetTaskLogsForCategory(
	accountID string,
	categoryID string,
) (
	[]*service.TaskLog,
	error,
) {
	rows, err := db.Conn.Query(taskLogSelectSQL()+`
		WHERE tl.account_id = ?1
			AND project.category_id = ?2
			AND tl.deleted_at IS NULL
		ORDER BY tl.created_at DESC`,
		accountID,
		categoryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task logs for category: %w", err)
	}
	defer rows.Close()
	return scanTaskLogRows(rows)
}

func (db *DB) GetProjectLogsForProject(
	accountID string,
	projectID string,
) (
	[]*service.ProjectLog,
	error,
) {
	rows, err := db.Conn.Query(projectLogSelectSQL()+`
		WHERE pl.account_id = ?1
			AND pl.project_id = ?2
			AND pl.deleted_at IS NULL
		ORDER BY pl.created_at DESC`,
		accountID,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project logs for project: %w", err)
	}
	defer rows.Close()
	return scanProjectLogRows(rows)
}

func logCreatedAt(
	customTime *time.Time,
) time.Time {
	if customTime != nil {
		return *customTime
	}
	return time.Now()
}

func (db *DB) sqlInsertTaskLogTx(
	tx *sql.Tx,
	id string,
	accountID string,
	taskID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	createdAt time.Time,
) (
	*service.TaskLog,
	error,
) {
	row := tx.QueryRow(`
		INSERT INTO task_log (
			id,
			account_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		)
		SELECT
			?1,
			?2,
			?3,
			?4,
			?5,
			?6,
			?7
		FROM tasks
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at,
			updated_at,
			deleted_at`,
		id,
		accountID,
		taskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		createdAt.Unix(),
	)

	taskLog, err := scanTaskLog(row)
	if err != nil {
		return nil, fmt.Errorf("insert task log: %w", err)
	}
	if err := tx.QueryRow(`
		SELECT t.project_id, p.category_id
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.account_id = ?1 AND t.id = ?2`,
		accountID,
		taskID,
	).Scan(&taskLog.ProjectID, &taskLog.CategoryID); err != nil {
		return nil, fmt.Errorf("read task log parents: %w", err)
	}

	return taskLog, nil
}

func (db *DB) sqlInsertProjectLogTx(
	tx *sql.Tx,
	id string,
	accountID string,
	projectID string,
	statusEstimate int,
	confidence string,
	note string,
	createdAt time.Time,
) (
	*service.ProjectLog,
	error,
) {
	row := tx.QueryRow(`
		INSERT INTO project_log (
			id,
			account_id,
			project_id,
			status_estimate,
			confidence,
			note,
			created_at
		)
		SELECT
			?1,
			?2,
			?3,
			?4,
			?5,
			?6,
			?7
		FROM projects
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			project_id,
			status_estimate,
			confidence,
			note,
			created_at,
			updated_at,
			deleted_at`,
		id,
		accountID,
		projectID,
		statusEstimate,
		confidence,
		note,
		createdAt.Unix(),
	)

	projectLog, err := scanProjectLog(row)
	if err != nil {
		return nil, fmt.Errorf("insert project log: %w", err)
	}
	if err := tx.QueryRow(`
		SELECT category_id
		FROM projects
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		projectID,
	).Scan(&projectLog.CategoryID); err != nil {
		return nil, fmt.Errorf("read project log category: %w", err)
	}

	return projectLog, nil
}

func (db *DB) sqlUpdateProjectStatusTx(
	tx *sql.Tx,
	accountID string,
	projectID string,
	statusEstimate int,
	confidence string,
) error {
	if _, err := tx.Exec(`
		UPDATE projects
		SET completion = ?1,
			confidence = ?2,
			updated_at = ?3
		WHERE account_id = ?4 AND id = ?5`,
		statusEstimate,
		confidence,
		time.Now().Unix(),
		accountID,
		projectID,
	); err != nil {
		return fmt.Errorf("update project status: %w", err)
	}
	return nil
}

func (db *DB) sqlUpdateTaskCompletionTx(
	tx *sql.Tx,
	accountID string,
	taskID string,
	completionEstimate int,
) error {
	if _, err := tx.Exec(`
		UPDATE tasks
		SET completion = ?1,
			updated_at = ?2
		WHERE account_id = ?3 AND id = ?4`,
		completionEstimate,
		time.Now().Unix(),
		accountID,
		taskID,
	); err != nil {
		return fmt.Errorf("update task completion: %w", err)
	}
	return nil
}

func taskLogSelectSQL() string {
	return `
		SELECT
			tl.id,
			project.category_id,
			task.project_id,
			tl.task_id,
			tl.hours_worked,
			tl.work_description,
			tl.completion_estimate,
			tl.created_at,
			tl.updated_at,
			tl.deleted_at
		FROM task_log tl
		JOIN tasks task ON tl.task_id = task.id AND tl.account_id = task.account_id
		JOIN projects project ON task.project_id = project.id AND task.account_id = project.account_id`
}

func projectLogSelectSQL() string {
	return `
		SELECT
			pl.id,
			project.category_id,
			pl.project_id,
			pl.status_estimate,
			pl.confidence,
			pl.note,
			pl.created_at,
			pl.updated_at,
			pl.deleted_at
		FROM project_log pl
		JOIN projects project ON pl.project_id = project.id AND pl.account_id = project.account_id`
}

func scanTaskLog(
	row *sql.Row,
) (
	*service.TaskLog,
	error,
) {
	var taskLog service.TaskLog
	var createdAt int64
	var updatedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := row.Scan(
		&taskLog.ID,
		&taskLog.TaskID,
		&taskLog.HoursWorked,
		&taskLog.WorkDescription,
		&taskLog.CompletionEstimate,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	taskLog.CreatedAt = time.Unix(createdAt, 0)
	if updatedAt.Valid {
		taskLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
	}
	if deletedAt.Valid {
		taskLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
	}
	return &taskLog, nil
}

func scanProjectLog(
	row *sql.Row,
) (
	*service.ProjectLog,
	error,
) {
	var projectLog service.ProjectLog
	var createdAt int64
	var updatedAt sql.NullInt64
	var deletedAt sql.NullInt64
	if err := row.Scan(
		&projectLog.ID,
		&projectLog.ProjectID,
		&projectLog.StatusEstimate,
		&projectLog.Confidence,
		&projectLog.Note,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	projectLog.CreatedAt = time.Unix(createdAt, 0)
	if updatedAt.Valid {
		projectLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
	}
	if deletedAt.Valid {
		projectLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
	}
	return &projectLog, nil
}

func scanTaskLogRows(
	rows *sql.Rows,
) (
	[]*service.TaskLog,
	error,
) {
	var logs []*service.TaskLog
	for rows.Next() {
		var taskLog service.TaskLog
		var createdAt int64
		var updatedAt sql.NullInt64
		var deletedAt sql.NullInt64
		if err := rows.Scan(
			&taskLog.ID,
			&taskLog.CategoryID,
			&taskLog.ProjectID,
			&taskLog.TaskID,
			&taskLog.HoursWorked,
			&taskLog.WorkDescription,
			&taskLog.CompletionEstimate,
			&createdAt,
			&updatedAt,
			&deletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task log row: %w", err)
		}
		taskLog.CreatedAt = time.Unix(createdAt, 0)
		if updatedAt.Valid {
			taskLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
		}
		if deletedAt.Valid {
			taskLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
		}
		logs = append(logs, &taskLog)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task log rows: %w", err)
	}
	return logs, nil
}

func scanProjectLogRows(
	rows *sql.Rows,
) (
	[]*service.ProjectLog,
	error,
) {
	var logs []*service.ProjectLog
	for rows.Next() {
		var projectLog service.ProjectLog
		var createdAt int64
		var updatedAt sql.NullInt64
		var deletedAt sql.NullInt64
		if err := rows.Scan(
			&projectLog.ID,
			&projectLog.CategoryID,
			&projectLog.ProjectID,
			&projectLog.StatusEstimate,
			&projectLog.Confidence,
			&projectLog.Note,
			&createdAt,
			&updatedAt,
			&deletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project log row: %w", err)
		}
		projectLog.CreatedAt = time.Unix(createdAt, 0)
		if updatedAt.Valid {
			projectLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
		}
		if deletedAt.Valid {
			projectLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
		}
		logs = append(logs, &projectLog)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project log rows: %w", err)
	}
	return logs, nil
}
