package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

var _ service.Store = (*SQLiteStore)(nil)

func NewSQLiteStore(
	path string,
	wal bool,
) (
	*SQLiteStore,
	error,
) {
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

func scanAccount(
	row *sql.Row,
) (
	*service.Account,
	error,
) {
	var account service.Account
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

func (s *SQLiteStore) GetAccountByHandle(
	handle string,
) (
	*service.Account,
	error,
) {
	return scanAccount(s.db.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE handle = ?1`,
		handle,
	))
}

func (s *SQLiteStore) GetAccountBySubject(
	subject string,
) (
	*service.Account,
	error,
) {
	return scanAccount(s.db.QueryRow(`
		SELECT id, consent_subject, handle, profile_refreshed_at, created_at, updated_at
		FROM accounts
		WHERE consent_subject = ?1`,
		subject,
	))
}

func (s *SQLiteStore) UpsertAccount(
	subject string,
	handle string,
	refreshedAt time.Time,
) (
	*service.Account,
	error,
) {
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

func (s *SQLiteStore) GetCategories(
	accountID string,
) (
	[]*service.Category,
	error,
) {
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

	var categories []*service.Category
	for categoryRows.Next() {
		var c service.Category
		if err := categoryRows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
			&c.Public,
		); err != nil {
			categoryRows.Close()
			return nil, err
		}
		c.Projects = []*service.Project{} // Initialize slice
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

	projectsByCat := make(map[string][]*service.Project)
	var allProjects []*service.Project
	for taskRows.Next() {
		var p service.Project
		if err := taskRows.Scan(
			&p.ID,
			&p.CategoryID,
			&p.Name,
			&p.Description,
			&p.Completion,
			&p.Public,
			&p.ParentPublic,
		); err != nil {
			taskRows.Close()
			return nil, err
		}
		p.Tasks = []*service.Task{}
		projectsByCat[p.CategoryID] = append(projectsByCat[p.CategoryID], &p)
		allProjects = append(allProjects, &p)
	}
	if err := taskRows.Err(); err != nil {
		taskRows.Close()
		return nil, err
	}
	if err := taskRows.Close(); err != nil {
		return nil, err
	}

	// get all tasks with parent public from projects and categories
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

	subsByTask := make(map[string][]*service.Task)
	for subRows.Next() {
		var sub service.Task
		if err := subRows.Scan(
			&sub.ID,
			&sub.ProjectID,
			&sub.CategoryID,
			&sub.Name,
			&sub.Description,
			&sub.Completion,
			&sub.Public,
			&sub.ParentPublic,
		); err != nil {
			return nil, err
		}
		subsByTask[sub.ProjectID] = append(subsByTask[sub.ProjectID], &sub)
	}
	if err := subRows.Err(); err != nil {
		subRows.Close()
		return nil, err
	}
	if err := subRows.Close(); err != nil {
		return nil, err
	}

	// Assemble
	for _, p := range allProjects {
		if subs, ok := subsByTask[p.ID]; ok {
			p.Tasks = subs
		}
	}

	for _, c := range categories {
		if projects, ok := projectsByCat[c.ID]; ok {
			c.Projects = projects
		}
	}

	return categories, nil
}

func (s *SQLiteStore) GetCategory(
	accountID string,
	id string,
) (
	*service.Category,
	error,
) {
	var c service.Category
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

	projects, err := s.getProjectsForCategory(accountID, c.ID)
	if err != nil {
		return nil, err
	}

	c.Projects = projects
	return &c, nil
}

func (s *SQLiteStore) getProjectsForCategory(
	accountID string,
	catID string,
) (
	[]*service.Project,
	error,
) {
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

	var projects []*service.Project
	for taskRows.Next() {
		var p service.Project
		if err := taskRows.Scan(
			&p.ID,
			&p.CategoryID,
			&p.Name,
			&p.Description,
			&p.Completion,
			&p.Public,
			&p.ParentPublic,
		); err != nil {
			taskRows.Close()
			return nil, err
		}

		projects = append(projects, &p)
	}
	if err := taskRows.Err(); err != nil {
		taskRows.Close()
		return nil, err
	}
	if err := taskRows.Close(); err != nil {
		return nil, err
	}

	for _, p := range projects {
		tasks, err := s.getTasksForProject(accountID, p.ID)
		if err != nil {
			return nil, err
		}
		p.Tasks = tasks
	}
	return projects, nil
}

func (s *SQLiteStore) getTasksForProject(
	accountID string,
	projectID string,
) (
	[]*service.Task,
	error,
) {
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
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer subtaskRows.Close()

	var subs []*service.Task
	for subtaskRows.Next() {
		var sub service.Task
		if err := subtaskRows.Scan(
			&sub.ID,
			&sub.ProjectID,
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

func (s *SQLiteStore) AddCategory(
	accountID string,
	name string,
) (
	*service.Category,
	error,
) {
	id := uuid.NewString()

	var minOrder sql.NullInt64
	s.db.QueryRow("SELECT MIN(sort_order) FROM categories WHERE account_id = ?1", accountID).Scan(&minOrder)
	order := int(minOrder.Int64) - 1

	var cat service.Category
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

	cat.Projects = []*service.Project{}
	return &cat, nil
}

func (s *SQLiteStore) UpdateCategory(
	accountID string,
	cat *service.Category,
) (
	*service.Category,
	error,
) {
	var updated service.Category
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

	projects, err := s.getProjectsForCategory(accountID, updated.ID)
	if err != nil {
		return nil, err
	}

	updated.Projects = projects
	return &updated, nil
}

func (s *SQLiteStore) DeleteCategory(
	accountID string,
	id string,
) (
	*service.Category,
	error,
) {
	var removed service.Category
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

func (s *SQLiteStore) ReorderCategories(
	accountID string,
	ids []string,
) error {
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

func (s *SQLiteStore) GetProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	var p service.Project
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
	tasks, err := s.getTasksForProject(accountID, p.ID)
	if err != nil {
		return nil, err
	}
	p.Tasks = tasks
	return &p, nil
}

func (s *SQLiteStore) AddProject(
	accountID string,
	catID string,
	name string,
) (
	*service.Project,
	error,
) {
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

	var project service.Project
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

func (s *SQLiteStore) UpdateProject(
	accountID string,
	project *service.Project,
) (
	*service.Project,
	error,
) {
	var updated service.Project
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

func (s *SQLiteStore) DeleteProject(
	accountID string,
	id string,
) (
	*service.Project,
	error,
) {
	var removed service.Project
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

func (s *SQLiteStore) ReorderProjects(
	accountID string,
	catID string,
	taskIDs []string,
) error {
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

func (s *SQLiteStore) GetTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	var sub service.Task
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

func (s *SQLiteStore) AddTask(
	accountID string,
	projectID string,
	name string,
) (
	*service.Task,
	error,
) {
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
		projectID,
	).Scan(&maxOrder); err != nil {
		return nil, err
	}
	order := int(maxOrder.Int64) + 1

	var sub service.Task
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

func (s *SQLiteStore) UpdateTask(
	accountID string,
	sub *service.Task,
) (
	*service.Task,
	error,
) {
	var updated service.Task
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

func (s *SQLiteStore) DeleteTask(
	accountID string,
	id string,
) (
	*service.Task,
	error,
) {
	var removed service.Task
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

func (s *SQLiteStore) ReorderTasks(
	accountID string,
	projectID string,
	taskIDs []string,
) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range taskIDs {
		if _, err := tx.Exec(`
			UPDATE subtasks
			SET sort_order = ?1
			WHERE account_id = ?2 AND id = ?3 AND task_id = ?4`,
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

func (s *SQLiteStore) AddWorkLogForProject(
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
	timestamp := time.Now()
	if customTime != nil {
		timestamp = *customTime
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var wl service.WorkLog
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
		projectID,
		hoursWorked,
		workDescription,
		completionEstimate,
		timestamp.Unix(),
	).Scan(
		&wl.ID,
		&wl.CategoryID,
		&wl.ProjectID,
		&subtaskIDNull,
		&wl.HoursWorked,
		&wl.WorkDescription,
		&wl.CompletionEstimate,
		&createdAtUnix,
	); err != nil {
		return nil, err
	}

	wl.TaskID = subtaskIDNull.String
	wl.CreatedAt = time.Unix(createdAtUnix, 0)

	if _, err := tx.Exec(`
		UPDATE tasks
		SET completion = ?1
		WHERE account_id = ?2 AND id = ?3`,
		completionEstimate,
		accountID,
		projectID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &wl, nil
}

func (s *SQLiteStore) AddWorkLogForTask(
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
	timestamp := time.Now()
	if customTime != nil {
		timestamp = *customTime
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var wl service.WorkLog
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
		taskID,
		hoursWorked,
		workDescription,
		completionEstimate,
		timestamp.Unix(),
	).Scan(
		&wl.ID,
		&wl.CategoryID,
		&wl.ProjectID,
		&subtaskIDNull,
		&wl.HoursWorked,
		&wl.WorkDescription,
		&wl.CompletionEstimate,
		&createdAtUnix,
	); err != nil {
		return nil, err
	}

	wl.TaskID = subtaskIDNull.String
	wl.CreatedAt = time.Unix(createdAtUnix, 0)

	if _, err := tx.Exec(`
		UPDATE subtasks
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

func (s *SQLiteStore) scanWorkLogs(
	rows *sql.Rows,
) (
	[]*service.WorkLog,
	error,
) {
	defer rows.Close()
	var logs []*service.WorkLog
	for rows.Next() {
		var wl service.WorkLog
		var createdAt int64
		var subtaskID sql.NullString
		if err := rows.Scan(
			&wl.ID,
			&wl.CategoryID,
			&wl.ProjectID,
			&subtaskID,
			&wl.HoursWorked,
			&wl.WorkDescription,
			&wl.CompletionEstimate,
			&createdAt,
		); err != nil {
			return nil, err
		}
		wl.TaskID = subtaskID.String
		wl.CreatedAt = time.Unix(createdAt, 0)
		logs = append(logs, &wl)
	}
	return logs, rows.Err()
}

func (s *SQLiteStore) GetWorkLogsForTask(
	accountID string,
	taskID string,
) (
	[]*service.WorkLog,
	error,
) {
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
		ORDER BY created_at DESC`, accountID, taskID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForProject(
	accountID string,
	projectID string,
) (
	[]*service.WorkLog,
	error,
) {
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
		projectID)
	if err != nil {
		return nil, err
	}
	return s.scanWorkLogs(rows)
}

func (s *SQLiteStore) GetWorkLogsForCategory(
	accountID string,
	categoryID string,
) (
	[]*service.WorkLog,
	error,
) {
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
