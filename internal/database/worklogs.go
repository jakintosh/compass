package database

import (
	"database/sql"
	"fmt"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
)

func (db *DB) AddWorkLogForProject(
	accountID string,
	projectID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	customTime *time.Time,
) (
	*service.WorkLog,
	error,
) {
	id := uuid.NewString()
	createdAt := workLogCreatedAt(customTime)

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add project work log tx: %w", err)
	}
	defer tx.Rollback()

	workLog, err := db.sqlInsertProjectWorkLogTx(
		tx,
		id,
		accountID,
		projectID,
		hoursWorked,
		workDescription,
		completionEstimate,
		createdAt,
	)
	if err != nil {
		return nil, err
	}
	if err := db.sqlUpdateProjectCompletionTx(tx, accountID, projectID, completionEstimate); err != nil {
		return nil, err
	}
	if err := db.sqlInsertEventTx(tx, accountID, "work_log", id, "work_log.created", fmt.Sprintf(`{"project_id":%q}`, projectID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add project work log tx: %w", err)
	}

	return workLog, nil
}

func (db *DB) AddWorkLogForTask(
	accountID string,
	taskID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	customTime *time.Time,
) (
	*service.WorkLog,
	error,
) {
	id := uuid.NewString()
	createdAt := workLogCreatedAt(customTime)

	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin add task work log tx: %w", err)
	}
	defer tx.Rollback()

	workLog, err := db.sqlInsertTaskWorkLogTx(
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
	if err := db.sqlInsertEventTx(tx, accountID, "work_log", id, "work_log.created", fmt.Sprintf(`{"task_id":%q}`, taskID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add task work log tx: %w", err)
	}

	return workLog, nil
}

func (db *DB) GetWorkLogsForTask(
	accountID string,
	taskID string,
) (
	[]*service.WorkLog,
	error,
) {
	rows, err := db.Conn.Query(workLogSelectSQL()+`
		WHERE wl.account_id = ?1
			AND wl.task_id = ?2
			AND wl.deleted_at IS NULL
		ORDER BY wl.created_at DESC`,
		accountID,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task work logs: %w", err)
	}
	defer rows.Close()
	return scanWorkLogRows(rows)
}

func (db *DB) GetWorkLogsForProject(
	accountID string,
	projectID string,
) (
	[]*service.WorkLog,
	error,
) {
	rows, err := db.Conn.Query(workLogSelectSQL()+`
		WHERE wl.account_id = ?1
			AND wl.deleted_at IS NULL
			AND (
				wl.project_id = ?2 OR
				task.project_id = ?2
			)
		ORDER BY wl.created_at DESC`,
		accountID,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project work logs: %w", err)
	}
	defer rows.Close()
	return scanWorkLogRows(rows)
}

func (db *DB) GetWorkLogsForCategory(
	accountID string,
	categoryID string,
) (
	[]*service.WorkLog,
	error,
) {
	rows, err := db.Conn.Query(workLogSelectSQL()+`
		WHERE wl.account_id = ?1
			AND wl.deleted_at IS NULL
			AND COALESCE(project.category_id, task_project.category_id) = ?2
		ORDER BY wl.created_at DESC`,
		accountID,
		categoryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list category work logs: %w", err)
	}
	defer rows.Close()
	return scanWorkLogRows(rows)
}

func workLogCreatedAt(
	customTime *time.Time,
) time.Time {
	if customTime != nil {
		return *customTime
	}
	return time.Now()
}

func (db *DB) sqlInsertProjectWorkLogTx(
	tx *sql.Tx,
	id string,
	accountID string,
	projectID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	createdAt time.Time,
) (
	*service.WorkLog,
	error,
) {
	row := tx.QueryRow(`
		INSERT INTO work_logs (
			id,
			account_id,
			project_id,
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
			NULL,
			?4,
			?5,
			?6,
			?7
		FROM projects
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			project_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at,
			updated_at,
			deleted_at`,
		id,
		accountID,
		projectID,
		hoursWorked,
		workDescription,
		completionEstimate,
		createdAt.Unix(),
	)

	workLog, err := scanWorkLog(row)
	if err != nil {
		return nil, fmt.Errorf("insert project work log: %w", err)
	}
	if err := tx.QueryRow(`
		SELECT category_id
		FROM projects
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		projectID,
	).Scan(&workLog.CategoryID); err != nil {
		return nil, fmt.Errorf("read project work log category: %w", err)
	}

	return workLog, nil
}

func (db *DB) sqlInsertTaskWorkLogTx(
	tx *sql.Tx,
	id string,
	accountID string,
	taskID string,
	hoursWorked float64,
	workDescription string,
	completionEstimate int,
	createdAt time.Time,
) (
	*service.WorkLog,
	error,
) {
	row := tx.QueryRow(`
		INSERT INTO work_logs (
			id,
			account_id,
			project_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		)
		SELECT
			?1,
			?2,
			NULL,
			?3,
			?4,
			?5,
			?6,
			?7
		FROM tasks
		WHERE account_id = ?2 AND id = ?3 AND deleted_at IS NULL
		RETURNING
			id,
			project_id,
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

	workLog, err := scanWorkLog(row)
	if err != nil {
		return nil, fmt.Errorf("insert task work log: %w", err)
	}
	if err := tx.QueryRow(`
		SELECT t.project_id, p.category_id
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.account_id = ?1 AND t.id = ?2`,
		accountID,
		taskID,
	).Scan(&workLog.ProjectID, &workLog.CategoryID); err != nil {
		return nil, fmt.Errorf("read task work log parents: %w", err)
	}

	return workLog, nil
}

func (db *DB) sqlUpdateProjectCompletionTx(
	tx *sql.Tx,
	accountID string,
	projectID string,
	completionEstimate int,
) error {
	if _, err := tx.Exec(`
		UPDATE projects
		SET completion = ?1,
			updated_at = ?2
		WHERE account_id = ?3 AND id = ?4`,
		completionEstimate,
		time.Now().Unix(),
		accountID,
		projectID,
	); err != nil {
		return fmt.Errorf("update project completion: %w", err)
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

func workLogSelectSQL() string {
	return `
		SELECT
			wl.id,
			COALESCE(project.category_id, task_project.category_id) AS category_id,
			COALESCE(wl.project_id, task.project_id) AS project_id,
			wl.task_id,
			wl.hours_worked,
			wl.work_description,
			wl.completion_estimate,
			wl.created_at,
			wl.updated_at,
			wl.deleted_at
		FROM work_logs wl
		LEFT JOIN projects project ON wl.project_id = project.id AND wl.account_id = project.account_id
		LEFT JOIN tasks task ON wl.task_id = task.id AND wl.account_id = task.account_id
		LEFT JOIN projects task_project ON task.project_id = task_project.id AND task.account_id = task_project.account_id`
}

func scanWorkLog(
	row *sql.Row,
) (
	*service.WorkLog,
	error,
) {
	var workLog service.WorkLog
	var createdAt int64
	var updatedAt sql.NullInt64
	var deletedAt sql.NullInt64
	var projectID sql.NullString
	var taskID sql.NullString
	if err := row.Scan(
		&workLog.ID,
		&projectID,
		&taskID,
		&workLog.HoursWorked,
		&workLog.WorkDescription,
		&workLog.CompletionEstimate,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	workLog.ProjectID = projectID.String
	workLog.TaskID = taskID.String
	workLog.CreatedAt = time.Unix(createdAt, 0)
	if updatedAt.Valid {
		workLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
	}
	if deletedAt.Valid {
		workLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
	}
	return &workLog, nil
}

func scanWorkLogRows(
	rows *sql.Rows,
) (
	[]*service.WorkLog,
	error,
) {
	var logs []*service.WorkLog
	for rows.Next() {
		var workLog service.WorkLog
		var createdAt int64
		var updatedAt sql.NullInt64
		var deletedAt sql.NullInt64
		var categoryID sql.NullString
		var projectID sql.NullString
		var taskID sql.NullString
		if err := rows.Scan(
			&workLog.ID,
			&categoryID,
			&projectID,
			&taskID,
			&workLog.HoursWorked,
			&workLog.WorkDescription,
			&workLog.CompletionEstimate,
			&createdAt,
			&updatedAt,
			&deletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan work log row: %w", err)
		}
		workLog.CategoryID = categoryID.String
		workLog.ProjectID = projectID.String
		workLog.TaskID = taskID.String
		workLog.CreatedAt = time.Unix(createdAt, 0)
		if updatedAt.Valid {
			workLog.UpdatedAt = time.Unix(updatedAt.Int64, 0)
		}
		if deletedAt.Valid {
			workLog.DeletedAt = time.Unix(deletedAt.Int64, 0)
		}
		logs = append(logs, &workLog)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work log rows: %w", err)
	}
	return logs, nil
}
