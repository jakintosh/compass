package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string, wal bool) (*SQLiteStore, error) {
	const busyTimeoutMS = 5000

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Serialize writes to avoid overlapping write transactions.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if wal {
		if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", busyTimeoutMS)); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			sort_order INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			category_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			completion INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS subtasks (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			category_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			completion INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS work_logs (
			id TEXT PRIMARY KEY,
			category_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			subtask_id TEXT,
			hours_worked REAL NOT NULL,
			work_description TEXT NOT NULL,
			completion_estimate INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE,
			FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY(subtask_id) REFERENCES subtasks(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_work_logs_category ON work_logs(category_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_task ON work_logs(task_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_subtask ON work_logs(subtask_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_created_at ON work_logs(created_at DESC);
	`)
	return err
}

func (s *SQLiteStore) GetCategories() ([]*domain.Category, error) {
	// get all categories
	categoryRows, err := s.db.Query(`
		SELECT
			id,
			name,
			description
		FROM categories
		ORDER BY sort_order ASC`,
	)
	if err != nil {
		return nil, err
	}

	var categories []*domain.Category
	for categoryRows.Next() {
		var c domain.Category
		if err := categoryRows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
		); err != nil {
			categoryRows.Close()
			return nil, err
		}
		c.Tasks = []*domain.Task{} // Initialize slice
		categories = append(categories, &c)
	}
	if err := categoryRows.Err(); err != nil {
		categoryRows.Close()
		return nil, err
	}
	if err := categoryRows.Close(); err != nil {
		return nil, err
	}

	// get all tasks
	taskRows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			name,
			description,
			completion
		FROM tasks
		ORDER BY sort_order ASC`,
	)
	if err != nil {
		return nil, err
	}

	tasksByCat := make(map[string][]*domain.Task)
	var allTasks []*domain.Task
	for taskRows.Next() {
		var t domain.Task
		if err := taskRows.Scan(
			&t.ID,
			&t.CategoryID,
			&t.Name,
			&t.Description,
			&t.Completion,
		); err != nil {
			taskRows.Close()
			return nil, err
		}
		t.Subtasks = []*domain.Subtask{}
		tasksByCat[t.CategoryID] = append(tasksByCat[t.CategoryID], &t)
		allTasks = append(allTasks, &t)
	}
	if err := taskRows.Err(); err != nil {
		taskRows.Close()
		return nil, err
	}
	if err := taskRows.Close(); err != nil {
		return nil, err
	}

	// get all subtasks
	subRows, err := s.db.Query(`
		SELECT
			id,
			task_id,
			category_id,
			name,
			description,
			completion
		FROM subtasks
		ORDER BY sort_order ASC`,
	)
	if err != nil {
		return nil, err
	}

	subsByTask := make(map[string][]*domain.Subtask)
	for subRows.Next() {
		var sub domain.Subtask
		if err := subRows.Scan(
			&sub.ID,
			&sub.TaskID,
			&sub.CategoryID,
			&sub.Name,
			&sub.Description,
			&sub.Completion,
		); err != nil {
			return nil, err
		}
		subsByTask[sub.TaskID] = append(subsByTask[sub.TaskID], &sub)
	}
	if err := subRows.Err(); err != nil {
		subRows.Close()
		return nil, err
	}
	if err := subRows.Close(); err != nil {
		return nil, err
	}

	// Assemble
	for _, t := range allTasks {
		if subs, ok := subsByTask[t.ID]; ok {
			t.Subtasks = subs
		}
	}

	for _, c := range categories {
		if tasks, ok := tasksByCat[c.ID]; ok {
			c.Tasks = tasks
		}
	}

	return categories, nil
}

func (s *SQLiteStore) GetCategory(id string) (*domain.Category, error) {
	var c domain.Category
	row := s.db.QueryRow(`
		SELECT
			id,
			name,
			description
		FROM categories
		WHERE id = ?1`,
		id,
	)
	if err := row.Scan(
		&c.ID,
		&c.Name,
		&c.Description,
	); err != nil {
		return nil, err
	}

	tasks, err := s.getTasksForCategory(c.ID)
	if err != nil {
		return nil, err
	}

	c.Tasks = tasks
	return &c, nil
}

func (s *SQLiteStore) getTasksForCategory(catID string) ([]*domain.Task, error) {
	taskRows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			name,
			description,
			completion
		FROM tasks
		WHERE category_id = ?1
		ORDER BY sort_order ASC`,
		catID,
	)
	if err != nil {
		return nil, err
	}

	var tasks []*domain.Task
	for taskRows.Next() {
		var t domain.Task
		if err := taskRows.Scan(
			&t.ID,
			&t.CategoryID,
			&t.Name,
			&t.Description,
			&t.Completion,
		); err != nil {
			taskRows.Close()
			return nil, err
		}

		tasks = append(tasks, &t)
	}
	if err := taskRows.Err(); err != nil {
		taskRows.Close()
		return nil, err
	}
	if err := taskRows.Close(); err != nil {
		return nil, err
	}

	for _, t := range tasks {
		subs, err := s.getSubtasksForTask(t.ID)
		if err != nil {
			return nil, err
		}
		t.Subtasks = subs
	}
	return tasks, nil
}

func (s *SQLiteStore) getSubtasksForTask(taskID string) ([]*domain.Subtask, error) {
	subtaskRows, err := s.db.Query(`
		SELECT
			id,
			task_id,
			category_id,
			name,
			description,
			completion
		FROM subtasks
		WHERE task_id = ?1
		ORDER BY sort_order ASC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer subtaskRows.Close()

	var subs []*domain.Subtask
	for subtaskRows.Next() {
		var sub domain.Subtask
		if err := subtaskRows.Scan(
			&sub.ID,
			&sub.TaskID,
			&sub.CategoryID,
			&sub.Name,
			&sub.Description,
			&sub.Completion,
		); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, nil
}

func (s *SQLiteStore) AddCategory(name string) (*domain.Category, error) {
	id := uuid.NewString()

	var minOrder sql.NullInt64
	s.db.QueryRow("SELECT MIN(sort_order) FROM categories").Scan(&minOrder)
	order := int(minOrder.Int64) - 1

	_, err := s.db.Exec(`
		INSERT INTO categories (id, name, sort_order)
		VALUES (?1, ?2, ?3)`,
		id,
		name,
		order,
	)
	if err != nil {
		return nil, err
	}

	return &domain.Category{
		ID:    id,
		Name:  name,
		Tasks: []*domain.Task{},
	}, nil
}

func (s *SQLiteStore) UpdateCategory(cat *domain.Category) (*domain.Category, error) {
	var updated domain.Category
	if err := s.db.QueryRow(
		`UPDATE categories
			SET name = ?1,
				description = ?2
			WHERE id = ?3
		RETURNING
			id,
			name,
			description`,
		cat.Name,
		cat.Description,
		cat.ID,
	).Scan(
		&updated.ID,
		&updated.Name,
		&updated.Description,
	); err != nil {
		return nil, err
	}
	updated.Tasks = cat.Tasks
	return &updated, nil
}

func (s *SQLiteStore) DeleteCategory(id string) (*domain.Category, error) {
	var removed domain.Category
	if err := s.db.QueryRow(`
		DELETE FROM categories
		WHERE id = ?1
		RETURNING
			id,
			name,
			description`,
		id,
	).Scan(
		&removed.ID,
		&removed.Name,
		&removed.Description,
	); err != nil {
		return nil, err
	}
	return &removed, nil
}

func (s *SQLiteStore) ReorderCategories(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`
			UPDATE categories
			SET sort_order = ?1
			WHERE id = ?2`,
			i,
			id,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetTask(id string) (*domain.Task, error) {
	var t domain.Task
	err := s.db.QueryRow(`
		SELECT
			id,
			category_id,
			name,
			description,
			completion
		FROM tasks
		WHERE id = ?1`,
		id,
	).Scan(
		&t.ID,
		&t.CategoryID,
		&t.Name,
		&t.Description,
		&t.Completion,
	)
	if err != nil {
		return nil, err
	}
	subs, err := s.getSubtasksForTask(t.ID)
	if err != nil {
		return nil, err
	}
	t.Subtasks = subs
	return &t, nil
}

func (s *SQLiteStore) AddTask(catID string, name string) (*domain.Task, error) {
	id := uuid.NewString()

	var maxOrder sql.NullInt64
	s.db.QueryRow(`
		SELECT MAX(sort_order)
		FROM tasks
		WHERE category_id = ?1`,
		catID,
	).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	if _, err := s.db.Exec(`
		INSERT INTO tasks (id, category_id, name, sort_order)
		VALUES (?1, ?2, ?3, ?4)`,
		id,
		catID,
		name,
		order,
	); err != nil {
		return nil, err
	}
	return &domain.Task{
		ID:         id,
		CategoryID: catID,
		Name:       name,
		Subtasks:   []*domain.Subtask{},
	}, nil
}

func (s *SQLiteStore) UpdateTask(task *domain.Task) (*domain.Task, error) {
	var updated domain.Task
	if err := s.db.QueryRow(`
		UPDATE tasks
		SET name = ?1,
			description = ?2,
			completion = ?3
		WHERE id = ?4
		RETURNING
			id,
			category_id,
			name,
			description,
			completion`,
		task.Name,
		task.Description,
		task.Completion,
		task.ID,
	).Scan(
		&updated.ID,
		&updated.CategoryID,
		&updated.Name,
		&updated.Description,
		&updated.Completion,
	); err != nil {
		return nil, err
	}
	updated.Subtasks = task.Subtasks
	return &updated, nil
}

func (s *SQLiteStore) DeleteTask(id string) (*domain.Task, error) {
	var removed domain.Task
	if err := s.db.QueryRow(`
		DELETE FROM tasks
		WHERE id = ?1
		RETURNING
			id,
			category_id,
			name,
			description,
			completion`,
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

func (s *SQLiteStore) ReorderTasks(catID string, taskIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range taskIDs {
		if _, err := tx.Exec(`
			UPDATE tasks
			SET sort_order = ?1
			WHERE id = ?2 AND category_id = ?3`,
			i,
			id,
			catID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetSubtask(id string) (*domain.Subtask, error) {
	var sub domain.Subtask
	err := s.db.QueryRow(
		`SELECT
			id,
			task_id,
			category_id,
			name,
			description,
			completion
		FROM subtasks
		WHERE id = ?1`,
		id,
	).Scan(
		&sub.ID,
		&sub.TaskID,
		&sub.CategoryID,
		&sub.Name,
		&sub.Description,
		&sub.Completion,
	)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *SQLiteStore) AddSubtask(taskID string, name string) (*domain.Subtask, error) {
	id := uuid.NewString()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var maxOrder sql.NullInt64
	if err := tx.QueryRow(`
		SELECT MAX(sort_order)
		FROM subtasks
		WHERE task_id = ?1`,
		taskID,
	).Scan(&maxOrder); err != nil {
		return nil, err
	}
	order := int(maxOrder.Int64) + 1

	var sub domain.Subtask
	if err := tx.QueryRow(`
		INSERT INTO subtasks (id, task_id, category_id, name, sort_order)
		SELECT ?1, ?2, category_id, ?3, ?4
		FROM tasks
		WHERE id = ?2
		RETURNING
			id,
			task_id,
			category_id,
			name,
			description,
			completion`,
		id,
		taskID,
		name,
		order,
	).Scan(
		&sub.ID,
		&sub.TaskID,
		&sub.CategoryID,
		&sub.Name,
		&sub.Description,
		&sub.Completion,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &sub, nil
}

func (s *SQLiteStore) UpdateSubtask(sub *domain.Subtask) (*domain.Subtask, error) {
	var updated domain.Subtask
	if err := s.db.QueryRow(`
		UPDATE subtasks
		SET name = ?1,
			description = ?2,
			completion = ?3
		WHERE id = ?4
		RETURNING
			id,
			task_id,
			category_id,
			name,
			description,
			completion`,
		sub.Name,
		sub.Description,
		sub.Completion,
		sub.ID,
	).Scan(
		&updated.ID,
		&updated.TaskID,
		&updated.CategoryID,
		&updated.Name,
		&updated.Description,
		&updated.Completion,
	); err != nil {
		return nil, err
	}

	return &updated, nil
}

func (s *SQLiteStore) DeleteSubtask(id string) (*domain.Subtask, error) {
	var removed domain.Subtask
	if err := s.db.QueryRow(`
		DELETE FROM subtasks
		WHERE id = ?1
		RETURNING
			id,
			task_id,
			category_id,
			name,
			description,
			completion`,
		id,
	).Scan(
		&removed.ID,
		&removed.TaskID,
		&removed.CategoryID,
		&removed.Name,
		&removed.Description,
		&removed.Completion,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("subtask not found")
		}
		return nil, err
	}

	return &removed, nil
}

func (s *SQLiteStore) ReorderSubtasks(taskID string, subIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range subIDs {
		if _, err := tx.Exec(`
			UPDATE subtasks
			SET sort_order = ?1
			WHERE id = ?2 AND task_id = ?3`,
			i,
			id,
			taskID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) AddWorkLogForTask(taskID string, hoursWorked float64, workDescription string, completionEstimate int) (*domain.WorkLog, error) {
	id := uuid.NewString()
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var wl domain.WorkLog
	var createdAtUnix int64
	var subtaskIDNull sql.NullString

	if err := tx.QueryRow(`
		INSERT INTO work_logs (
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at)
		SELECT
			?1,
			category_id,
			?2,
			NULL,
			?3,
			?4,
			?5,
			?6
		FROM tasks
		WHERE id = ?2
		RETURNING
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at`,
		id,
		taskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		now.Unix(),
	).Scan(
		&wl.ID,
		&wl.CategoryID,
		&wl.TaskID,
		&subtaskIDNull,
		&wl.HoursWorked,
		&wl.WorkDescription,
		&wl.CompletionEstimate,
		&createdAtUnix,
	); err != nil {
		return nil, err
	}

	wl.SubtaskID = subtaskIDNull.String
	wl.CreatedAt = time.Unix(createdAtUnix, 0)

	if _, err := tx.Exec(`
		UPDATE tasks
		SET completion = ?1
		WHERE id = ?2`,
		completionEstimate,
		taskID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &wl, nil
}

func (s *SQLiteStore) AddWorkLogForSubtask(subtaskID string, hoursWorked float64, workDescription string, completionEstimate int) (*domain.WorkLog, error) {
	id := uuid.NewString()
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var wl domain.WorkLog
	var createdAtUnix int64
	var subtaskIDNull sql.NullString

	if err := tx.QueryRow(`
		INSERT INTO work_logs (
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		)
		SELECT
			?1,
			category_id,
			task_id,
			?2,
			?3,
			?4,
			?5,
			?6
		FROM subtasks
		WHERE id = ?2
		RETURNING
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at`,
		id,
		subtaskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		now.Unix(),
	).Scan(
		&wl.ID,
		&wl.CategoryID,
		&wl.TaskID,
		&subtaskIDNull,
		&wl.HoursWorked,
		&wl.WorkDescription,
		&wl.CompletionEstimate,
		&createdAtUnix,
	); err != nil {
		return nil, err
	}

	wl.SubtaskID = subtaskIDNull.String
	wl.CreatedAt = time.Unix(createdAtUnix, 0)

	if _, err := tx.Exec(`
		UPDATE subtasks
		SET completion = ?1
		WHERE id = ?2`,
		completionEstimate,
		subtaskID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &wl, nil
}

func (s *SQLiteStore) scanWorkLogs(rows *sql.Rows) ([]*domain.WorkLog, error) {
	defer rows.Close()
	var logs []*domain.WorkLog
	for rows.Next() {
		var wl domain.WorkLog
		var createdAt int64
		var subtaskID sql.NullString
		if err := rows.Scan(
			&wl.ID,
			&wl.CategoryID,
			&wl.TaskID,
			&subtaskID,
			&wl.HoursWorked,
			&wl.WorkDescription,
			&wl.CompletionEstimate,
			&createdAt,
		); err != nil {
			return nil, err
		}
		wl.SubtaskID = subtaskID.String
		wl.CreatedAt = time.Unix(createdAt, 0)
		logs = append(logs, &wl)
	}
	return logs, rows.Err()
}

func (s *SQLiteStore) GetWorkLogsForSubtask(subtaskID string) ([]*domain.WorkLog, error) {
	rows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		FROM work_logs
		WHERE subtask_id = ?1
		ORDER BY created_at DESC`, subtaskID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForTask(taskID string) ([]*domain.WorkLog, error) {
	rows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		FROM work_logs
		WHERE task_id = ?1
		ORDER BY created_at DESC`,
		taskID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForCategory(categoryID string) ([]*domain.WorkLog, error) {
	rows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at
		FROM work_logs
		WHERE category_id = ?1
		ORDER BY created_at DESC`,
		categoryID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}
