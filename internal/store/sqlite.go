package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			consent_subject TEXT NOT NULL UNIQUE,
			handle TEXT NOT NULL UNIQUE,
			profile_refreshed_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			public INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			category_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			completion INTEGER DEFAULT 0,
			public INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS subtasks (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			task_id TEXT NOT NULL,
			category_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			completion INTEGER DEFAULT 0,
			public INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS work_logs (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			category_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			subtask_id TEXT,
			hours_worked REAL NOT NULL,
			work_description TEXT NOT NULL,
			completion_estimate INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE,
			FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY(subtask_id) REFERENCES subtasks(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_categories_account ON categories(account_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_account ON tasks(account_id);
		CREATE INDEX IF NOT EXISTS idx_subtasks_account ON subtasks(account_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_account ON work_logs(account_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_category ON work_logs(category_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_task ON work_logs(task_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_subtask ON work_logs(subtask_id);
		CREATE INDEX IF NOT EXISTS idx_work_logs_created_at ON work_logs(created_at DESC);
	`)
	if err != nil {
		return err
	}

	for _, stmt := range []string{
		"ALTER TABLE categories ADD COLUMN account_id TEXT",
		"ALTER TABLE tasks ADD COLUMN account_id TEXT",
		"ALTER TABLE subtasks ADD COLUMN account_id TEXT",
		"ALTER TABLE work_logs ADD COLUMN account_id TEXT",
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}

func scanAccount(row *sql.Row) (*domain.Account, error) {
	var account domain.Account
	var profileRefreshedAt int64
	var createdAt int64
	var updatedAt int64
	if err := row.Scan(
		&account.ID,
		&account.ConsentSubject,
		&account.Handle,
		&profileRefreshedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	account.ProfileRefreshedAt = time.Unix(profileRefreshedAt, 0)
	account.CreatedAt = time.Unix(createdAt, 0)
	account.UpdatedAt = time.Unix(updatedAt, 0)
	return &account, nil
}

func (s *SQLiteStore) GetAccountByHandle(handle string) (*domain.Account, error) {
	return scanAccount(s.db.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE handle = ?1`,
		handle,
	))
}

func (s *SQLiteStore) GetAccountBySubject(subject string) (*domain.Account, error) {
	return scanAccount(s.db.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE consent_subject = ?1`,
		subject,
	))
}

func (s *SQLiteStore) UpsertAccount(subject string, handle string, refreshedAt time.Time) (*domain.Account, error) {
	id := uuid.NewString()
	now := time.Now().Unix()
	refreshed := refreshedAt.Unix()
	return scanAccount(s.db.QueryRow(`
		INSERT INTO accounts (id, consent_subject, handle, profile_refreshed_at, created_at, updated_at)
		VALUES (?1, ?2, ?3, ?4, ?5, ?5)
		ON CONFLICT(consent_subject) DO UPDATE SET
			handle = excluded.handle,
			profile_refreshed_at = excluded.profile_refreshed_at,
			updated_at = ?5
		RETURNING id, consent_subject, handle, profile_refreshed_at, created_at, updated_at`,
		id,
		subject,
		handle,
		refreshed,
		now,
	))
}

func (s *SQLiteStore) GetCategories(accountID string) ([]*domain.Category, error) {
	// get all categories
	categoryRows, err := s.db.Query(`
		SELECT
			id,
			name,
			description,
			public
		FROM categories
		WHERE account_id = ?1
		ORDER BY sort_order ASC`,
		accountID,
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
			&c.Public,
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

	// get all tasks with parent public from categories
	taskRows, err := s.db.Query(`
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
		WHERE t.account_id = ?1
		ORDER BY t.sort_order ASC`,
		accountID,
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
			&t.Public,
			&t.ParentPublic,
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

	// get all subtasks with parent public from tasks and categories
	subRows, err := s.db.Query(`
		SELECT
			s.id,
			s.task_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM subtasks s
		JOIN tasks t ON s.task_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1
		ORDER BY s.sort_order ASC`,
		accountID,
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
			&sub.Public,
			&sub.ParentPublic,
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

func (s *SQLiteStore) GetCategory(accountID string, id string) (*domain.Category, error) {
	var c domain.Category
	row := s.db.QueryRow(`
		SELECT
			id,
			name,
			description,
			public
		FROM categories
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		id,
	)
	if err := row.Scan(
		&c.ID,
		&c.Name,
		&c.Description,
		&c.Public,
	); err != nil {
		return nil, err
	}

	tasks, err := s.getTasksForCategory(accountID, c.ID)
	if err != nil {
		return nil, err
	}

	c.Tasks = tasks
	return &c, nil
}

func (s *SQLiteStore) getTasksForCategory(accountID string, catID string) ([]*domain.Task, error) {
	taskRows, err := s.db.Query(`
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
		WHERE t.account_id = ?1 AND t.category_id = ?2
		ORDER BY t.sort_order ASC`,
		accountID,
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
			&t.Public,
			&t.ParentPublic,
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
		subs, err := s.getSubtasksForTask(accountID, t.ID)
		if err != nil {
			return nil, err
		}
		t.Subtasks = subs
	}
	return tasks, nil
}

func (s *SQLiteStore) getSubtasksForTask(accountID string, taskID string) ([]*domain.Subtask, error) {
	subtaskRows, err := s.db.Query(`
		SELECT
			s.id,
			s.task_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM subtasks s
		JOIN tasks t ON s.task_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1 AND s.task_id = ?2
		ORDER BY s.sort_order ASC`,
		accountID,
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
			&sub.Public,
			&sub.ParentPublic,
		); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, nil
}

func (s *SQLiteStore) AddCategory(accountID string, name string) (*domain.Category, error) {
	id := uuid.NewString()

	var minOrder sql.NullInt64
	s.db.QueryRow("SELECT MIN(sort_order) FROM categories WHERE account_id = ?1", accountID).Scan(&minOrder)
	order := int(minOrder.Int64) - 1

	var cat domain.Category
	if err := s.db.QueryRow(`
		INSERT INTO categories (id, account_id, name, sort_order)
		VALUES (?1, ?2, ?3, ?4)
		RETURNING
			id,
			name,
			description,
			public`,
		id,
		accountID,
		name,
		order,
	).Scan(
		&cat.ID,
		&cat.Name,
		&cat.Description,
		&cat.Public,
	); err != nil {
		return nil, err
	}

	cat.Tasks = []*domain.Task{}
	return &cat, nil
}

func (s *SQLiteStore) UpdateCategory(accountID string, cat *domain.Category) (*domain.Category, error) {
	var updated domain.Category
	if err := s.db.QueryRow(
		`UPDATE categories
			SET name = ?1,
				description = ?2,
				public = ?3
			WHERE account_id = ?4 AND id = ?5
		RETURNING
			id,
			name,
			description,
			public`,
		cat.Name,
		cat.Description,
		cat.Public,
		accountID,
		cat.ID,
	).Scan(
		&updated.ID,
		&updated.Name,
		&updated.Description,
		&updated.Public,
	); err != nil {
		return nil, err
	}

	tasks, err := s.getTasksForCategory(accountID, updated.ID)
	if err != nil {
		return nil, err
	}

	updated.Tasks = tasks
	return &updated, nil
}

func (s *SQLiteStore) DeleteCategory(accountID string, id string) (*domain.Category, error) {
	var removed domain.Category
	if err := s.db.QueryRow(`
		DELETE FROM categories
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			name,
			description`,
		accountID,
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

func (s *SQLiteStore) ReorderCategories(accountID string, ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`
			UPDATE categories
			SET sort_order = ?1
			WHERE account_id = ?2 AND id = ?3`,
			i,
			accountID,
			id,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetTask(accountID string, id string) (*domain.Task, error) {
	var t domain.Task
	err := s.db.QueryRow(`
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
		&t.ID,
		&t.CategoryID,
		&t.Name,
		&t.Description,
		&t.Completion,
		&t.Public,
		&t.ParentPublic,
	)
	if err != nil {
		return nil, err
	}
	subs, err := s.getSubtasksForTask(accountID, t.ID)
	if err != nil {
		return nil, err
	}
	t.Subtasks = subs
	return &t, nil
}

func (s *SQLiteStore) AddTask(accountID string, catID string, name string) (*domain.Task, error) {
	id := uuid.NewString()

	var maxOrder sql.NullInt64
	s.db.QueryRow(`
		SELECT MAX(sort_order)
		FROM tasks
		WHERE account_id = ?1 AND category_id = ?2`,
		accountID,
		catID,
	).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	var task domain.Task
	if err := s.db.QueryRow(`
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
		&task.ID,
		&task.CategoryID,
		&task.Name,
		&task.Description,
		&task.Completion,
		&task.Public,
	); err != nil {
		return nil, err
	}

	task.Subtasks = []*domain.Subtask{}
	return &task, nil
}

func (s *SQLiteStore) UpdateTask(accountID string, task *domain.Task) (*domain.Task, error) {
	var updated domain.Task
	if err := s.db.QueryRow(`
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
		task.Name,
		task.Description,
		task.Completion,
		task.Public,
		accountID,
		task.ID,
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
	updated.Subtasks = task.Subtasks
	return &updated, nil
}

func (s *SQLiteStore) DeleteTask(accountID string, id string) (*domain.Task, error) {
	var removed domain.Task
	if err := s.db.QueryRow(`
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

func (s *SQLiteStore) ReorderTasks(accountID string, catID string, taskIDs []string) error {
	tx, err := s.db.Begin()
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

func (s *SQLiteStore) GetSubtask(accountID string, id string) (*domain.Subtask, error) {
	var sub domain.Subtask
	err := s.db.QueryRow(
		`SELECT
			s.id,
			s.task_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM subtasks s
		JOIN tasks t ON s.task_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1 AND s.id = ?2`,
		accountID,
		id,
	).Scan(
		&sub.ID,
		&sub.TaskID,
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

func (s *SQLiteStore) AddSubtask(accountID string, taskID string, name string) (*domain.Subtask, error) {
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
		WHERE account_id = ?1 AND task_id = ?2`,
		accountID,
		taskID,
	).Scan(&maxOrder); err != nil {
		return nil, err
	}
	order := int(maxOrder.Int64) + 1

	var sub domain.Subtask
	if err := tx.QueryRow(`
		INSERT INTO subtasks (id, account_id, task_id, category_id, name, sort_order)
		SELECT ?1, ?2, id, category_id, ?4, ?5
		FROM tasks
		WHERE account_id = ?2 AND id = ?3
		RETURNING
			id,
			task_id,
			category_id,
			name,
			description,
			completion,
			public`,
		id,
		accountID,
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
		&sub.Public,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &sub, nil
}

func (s *SQLiteStore) UpdateSubtask(accountID string, sub *domain.Subtask) (*domain.Subtask, error) {
	var updated domain.Subtask
	if err := s.db.QueryRow(`
		UPDATE subtasks
		SET name = ?1,
			description = ?2,
			completion = ?3,
			public = ?4
		WHERE account_id = ?5 AND id = ?6
		RETURNING
			id,
			task_id,
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
		&updated.TaskID,
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

func (s *SQLiteStore) DeleteSubtask(accountID string, id string) (*domain.Subtask, error) {
	var removed domain.Subtask
	if err := s.db.QueryRow(`
		DELETE FROM subtasks
		WHERE account_id = ?1 AND id = ?2
		RETURNING
			id,
			task_id,
			category_id,
			name,
			description,
			completion`,
		accountID,
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

func (s *SQLiteStore) ReorderSubtasks(accountID string, taskID string, subIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range subIDs {
		if _, err := tx.Exec(`
			UPDATE subtasks
			SET sort_order = ?1
			WHERE account_id = ?2 AND id = ?3 AND task_id = ?4`,
			i,
			accountID,
			id,
			taskID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) AddWorkLogForTask(accountID string, taskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*domain.WorkLog, error) {
	id := uuid.NewString()
	timestamp := time.Now()
	if customTime != nil {
		timestamp = *customTime
	}

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
			account_id,
			category_id,
			task_id,
			subtask_id,
			hours_worked,
			work_description,
			completion_estimate,
			created_at)
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
		FROM tasks
		WHERE account_id = ?2 AND id = ?3
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
		accountID,
		taskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		timestamp.Unix(),
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
		WHERE account_id = ?2 AND id = ?3`,
		completionEstimate,
		accountID,
		taskID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &wl, nil
}

func (s *SQLiteStore) AddWorkLogForSubtask(accountID string, subtaskID string, hoursWorked float64, workDescription string, completionEstimate int, customTime *time.Time) (*domain.WorkLog, error) {
	id := uuid.NewString()
	timestamp := time.Now()
	if customTime != nil {
		timestamp = *customTime
	}

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
			account_id,
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
			?2,
			category_id,
			task_id,
			?3,
			?4,
			?5,
			?6,
			?7
		FROM subtasks
		WHERE account_id = ?2 AND id = ?3
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
		accountID,
		subtaskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		timestamp.Unix(),
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
		WHERE account_id = ?2 AND id = ?3`,
		completionEstimate,
		accountID,
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

func (s *SQLiteStore) GetWorkLogsForSubtask(accountID string, subtaskID string) ([]*domain.WorkLog, error) {
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
		WHERE account_id = ?1 AND subtask_id = ?2
		ORDER BY created_at DESC`, accountID, subtaskID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForTask(accountID string, taskID string) ([]*domain.WorkLog, error) {
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
		WHERE account_id = ?1 AND task_id = ?2
		ORDER BY created_at DESC`,
		accountID,
		taskID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForCategory(accountID string, categoryID string) ([]*domain.WorkLog, error) {
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
		WHERE account_id = ?1 AND category_id = ?2
		ORDER BY created_at DESC`,
		accountID,
		categoryID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}
