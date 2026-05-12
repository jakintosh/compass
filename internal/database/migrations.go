package database

import "fmt"

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "create compass schema",
		SQL: `
			CREATE TABLE accounts (
				id TEXT PRIMARY KEY,
				consent_subject TEXT NOT NULL UNIQUE,
				handle TEXT NOT NULL UNIQUE,
				profile_refreshed_at INTEGER NOT NULL,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			);

			CREATE TABLE categories (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				public INTEGER NOT NULL DEFAULT 1,
				sort_order INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
			);

			CREATE TABLE tasks (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				category_id TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				completion INTEGER NOT NULL DEFAULT 0,
				public INTEGER NOT NULL DEFAULT 1,
				sort_order INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
			);

			CREATE TABLE subtasks (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				task_id TEXT NOT NULL,
				category_id TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				completion INTEGER NOT NULL DEFAULT 0,
				public INTEGER NOT NULL DEFAULT 1,
				sort_order INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
				FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
			);

			CREATE TABLE work_logs (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
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

			CREATE INDEX idx_categories_account ON categories(account_id);
			CREATE INDEX idx_tasks_account ON tasks(account_id);
			CREATE INDEX idx_subtasks_account ON subtasks(account_id);
			CREATE INDEX idx_work_logs_account ON work_logs(account_id);
			CREATE INDEX idx_work_logs_category ON work_logs(category_id);
			CREATE INDEX idx_work_logs_task ON work_logs(task_id);
			CREATE INDEX idx_work_logs_subtask ON work_logs(subtask_id);
			CREATE INDEX idx_work_logs_created_at ON work_logs(created_at DESC);
		`,
	},
}

func (db *DB) migrate() error {
	current, err := db.userVersion()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}

		tx, err := db.Conn.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d %q: %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("run migration %d %q: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("set schema version %d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d %q: %w", m.Version, m.Name, err)
		}

		current = m.Version
	}

	return nil
}

func (db *DB) userVersion() (int, error) {
	var version int
	if err := db.Conn.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
