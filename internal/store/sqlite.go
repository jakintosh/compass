package store

import (
	"database/sql"
	"errors"
	"fmt"

	"git.sr.ht/~jakintosh/todo/internal/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
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
	`)
	return err
}

func (s *SQLiteStore) GetCategories() ([]*domain.Category, error) {
	rows, err := s.db.Query(`
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
	defer rows.Close()

	// get all categories
	var categories []*domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
		); err != nil {
			return nil, err
		}
		c.Tasks = []*domain.Task{} // Initialize slice
		categories = append(categories, &c)
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
	defer taskRows.Close()

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
			return nil, err
		}
		t.Subtasks = []*domain.Subtask{}
		tasksByCat[t.CategoryID] = append(tasksByCat[t.CategoryID], &t)
		allTasks = append(allTasks, &t)
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
	defer subRows.Close()

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
		WHERE id = ?`,
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
	rows, err := s.db.Query(`
		SELECT
			id,
			category_id,
			name,
			description,
			completion
		FROM tasks
		WHERE category_id = ?
		ORDER BY sort_order ASC`,
		catID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(
			&t.ID,
			&t.CategoryID,
			&t.Name,
			&t.Description,
			&t.Completion,
		); err != nil {
			return nil, err
		}

		subs, err := s.getSubtasksForTask(t.ID)
		if err != nil {
			return nil, err
		}
		t.Subtasks = subs

		tasks = append(tasks, &t)
	}
	return tasks, nil
}

func (s *SQLiteStore) getSubtasksForTask(taskID string) ([]*domain.Subtask, error) {
	rows, err := s.db.Query(`
		SELECT
			id,
			task_id,
			category_id,
			name,
			description,
			completion
		FROM subtasks
		WHERE task_id = ?
		ORDER BY sort_order ASC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subtask
	for rows.Next() {
		var sub domain.Subtask
		if err := rows.Scan(
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
		VALUES (?, ?, ?)`,
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
			SET name = ?,
				description = ?
			WHERE id = ?
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
		WHERE id = ?
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
			SET sort_order = ?
			WHERE id = ?`,
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
		WHERE id = ?`,
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
		WHERE category_id = ?`,
		catID,
	).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	if _, err := s.db.Exec(`
		INSERT INTO tasks (id, category_id, name, sort_order)
		VALUES (?, ?, ?, ?)`,
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
		SET name = ?,
			description = ?,
			completion = ?
		WHERE id = ?
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
		WHERE id = ?
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
			SET sort_order = ?
			WHERE id = ? AND category_id = ?`,
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
		WHERE id = ?`,
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
		WHERE task_id = ?`,
		taskID,
	).Scan(&maxOrder); err != nil {
		return nil, err
	}
	order := int(maxOrder.Int64) + 1

	var sub domain.Subtask
	if err := tx.QueryRow(`
		INSERT INTO subtasks (id, task_id, category_id, name, sort_order)
		SELECT ?, ?, category_id, ?, ?
		FROM tasks
		WHERE id = ?
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
		taskID,
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

	if err := updateTaskCompletionTx(tx, sub.TaskID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &sub, nil
}

func (s *SQLiteStore) UpdateSubtask(sub *domain.Subtask) (*domain.Subtask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var updated domain.Subtask
	if err := tx.QueryRow(`
		UPDATE subtasks
		SET name = ?,
			description = ?,
			completion = ?
		WHERE id = ?
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

	if err := updateTaskCompletionTx(tx, updated.TaskID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &updated, nil
}

func (s *SQLiteStore) DeleteSubtask(id string) (*domain.Subtask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var removed domain.Subtask
	if err := tx.QueryRow(`
		DELETE FROM subtasks
		WHERE id = ?
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

	if err := updateTaskCompletionTx(tx, removed.TaskID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
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
			SET sort_order = ?
			WHERE id = ? AND task_id = ?`,
			i,
			id,
			taskID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type taskCompletionExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func updateTaskCompletionTx(exec taskCompletionExecutor, taskID string) error {
	_, err := exec.Exec(`
		UPDATE tasks
		SET completion = COALESCE(
			(
				SELECT CAST(AVG(completion) AS INTEGER)
				FROM subtasks
				WHERE task_id = ?
			),
			0)
		WHERE id = ?`,
		taskID,
		taskID,
	)
	return err
}
