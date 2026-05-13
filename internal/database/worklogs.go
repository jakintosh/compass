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
	return db.getWorkLogs(
		`task_id = ?2`,
		accountID,
		taskID,
	)
}

func (db *DB) GetWorkLogsForProject(
	accountID string,
	projectID string,
) (
	[]*service.WorkLog,
	error,
) {
	return db.getWorkLogs(
		`project_id = ?2`,
		accountID,
		projectID,
	)
}

func (db *DB) GetWorkLogsForCategory(
	accountID string,
	categoryID string,
) (
	[]*service.WorkLog,
	error,
) {
	return db.getWorkLogs(
		`category_id = ?2`,
		accountID,
		categoryID,
	)
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
			category_id,
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
			category_id,
			?3,
			NULL,
			?4,
			?5,
			?6,
			?7
		FROM projects
		WHERE account_id = ?2 AND id = ?3
		RETURNING
			id,
			category_id,
			project_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at`,
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
			category_id,
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
			category_id,
			project_id,
			?3,
			?4,
			?5,
			?6,
			?7
		FROM tasks
		WHERE account_id = ?2 AND id = ?3
		RETURNING
			id,
			category_id,
			project_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at`,
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
		SET completion = ?1
		WHERE account_id = ?2 AND id = ?3`,
		completionEstimate,
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
		SET completion = ?1
		WHERE account_id = ?2 AND id = ?3`,
		completionEstimate,
		accountID,
		taskID,
	); err != nil {
		return fmt.Errorf("update task completion: %w", err)
	}
	return nil
}

func (db *DB) getWorkLogs(
	filter string,
	accountID string,
	parentID string,
) (
	[]*service.WorkLog,
	error,
) {
	rows, err := db.Conn.Query(fmt.Sprintf(`
		SELECT
			id,
			category_id,
			project_id,
			task_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		FROM work_logs
		WHERE account_id = ?1 AND %s
		ORDER BY created_at DESC`,
		filter,
	),
		accountID,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list work logs: %w", err)
	}
	defer rows.Close()

	return scanWorkLogRows(rows)
}

func scanWorkLog(
	row *sql.Row,
) (
	*service.WorkLog,
	error,
) {
	var workLog service.WorkLog
	var createdAt int64
	var taskID sql.NullString
	if err := row.Scan(
		&workLog.ID,
		&workLog.CategoryID,
		&workLog.ProjectID,
		&taskID,
		&workLog.HoursWorked,
		&workLog.WorkDescription,
		&workLog.CompletionEstimate,
		&createdAt,
	); err != nil {
		return nil, err
	}
	workLog.TaskID = taskID.String
	workLog.CreatedAt = time.Unix(createdAt, 0)
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
		var taskID sql.NullString
		if err := rows.Scan(
			&workLog.ID,
			&workLog.CategoryID,
			&workLog.ProjectID,
			&taskID,
			&workLog.HoursWorked,
			&workLog.WorkDescription,
			&workLog.CompletionEstimate,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan work log row: %w", err)
		}
		workLog.TaskID = taskID.String
		workLog.CreatedAt = time.Unix(createdAt, 0)
		logs = append(logs, &workLog)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work log rows: %w", err)
	}
	return logs, nil
}
