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
				status TEXT NOT NULL DEFAULT 'active',
				public INTEGER NOT NULL DEFAULT 1 CHECK (public IN (0, 1)),
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL,
				archived_at INTEGER,
				deleted_at INTEGER,
				UNIQUE(account_id, id),
				CHECK (status IN ('active', 'archived', 'deleted')),
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
			);

			CREATE TABLE projects (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				category_id TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				completion INTEGER NOT NULL DEFAULT 0 CHECK (completion BETWEEN 0 AND 100),
				public INTEGER NOT NULL DEFAULT 1 CHECK (public IN (0, 1)),
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL,
				archived_at INTEGER,
				deleted_at INTEGER,
				UNIQUE(account_id, id),
				CHECK (status IN ('active', 'archived', 'deleted')),
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(account_id, category_id) REFERENCES categories(account_id, id) ON DELETE CASCADE
			);

			CREATE TABLE tasks (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				project_id TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				completion INTEGER NOT NULL DEFAULT 0 CHECK (completion BETWEEN 0 AND 100),
				public INTEGER NOT NULL DEFAULT 1 CHECK (public IN (0, 1)),
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL,
				archived_at INTEGER,
				deleted_at INTEGER,
				UNIQUE(account_id, id),
				CHECK (status IN ('active', 'archived', 'deleted')),
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(account_id, project_id) REFERENCES projects(account_id, id) ON DELETE CASCADE
			);

			CREATE TABLE work_logs (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				project_id TEXT,
				task_id TEXT,
				hours_worked REAL NOT NULL CHECK (hours_worked >= 0),
				work_description TEXT NOT NULL,
				completion_estimate INTEGER NOT NULL CHECK (completion_estimate BETWEEN 0 AND 100),
				created_at INTEGER NOT NULL,
				updated_at INTEGER,
				deleted_at INTEGER,
				CHECK (
					(project_id IS NOT NULL AND task_id IS NULL) OR
					(project_id IS NULL AND task_id IS NOT NULL)
				),
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(account_id, project_id) REFERENCES projects(account_id, id) ON DELETE CASCADE,
				FOREIGN KEY(account_id, task_id) REFERENCES tasks(account_id, id) ON DELETE CASCADE
			);

			CREATE TABLE entity_events (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				actor_account_id TEXT,
				entity_type TEXT NOT NULL,
				entity_id TEXT NOT NULL,
				event_type TEXT NOT NULL,
				occurred_at INTEGER NOT NULL,
				data_json TEXT NOT NULL DEFAULT '{}',
				CHECK (entity_type IN ('category', 'project', 'task', 'work_log')),
				CHECK (actor_account_id IS NULL OR actor_account_id = account_id),
				FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
				FOREIGN KEY(actor_account_id) REFERENCES accounts(id) ON DELETE SET NULL
			);

			CREATE INDEX idx_categories_account ON categories(account_id);
			CREATE INDEX idx_projects_account ON projects(account_id);
			CREATE INDEX idx_projects_category ON projects(category_id);
			CREATE INDEX idx_tasks_account ON tasks(account_id);
			CREATE INDEX idx_tasks_project ON tasks(project_id);
			CREATE INDEX idx_work_logs_account ON work_logs(account_id);
			CREATE INDEX idx_work_logs_project ON work_logs(project_id);
			CREATE INDEX idx_work_logs_task ON work_logs(task_id);
			CREATE INDEX idx_work_logs_created_at ON work_logs(created_at DESC);
			CREATE INDEX idx_entity_events_account ON entity_events(account_id);
			CREATE INDEX idx_entity_events_entity ON entity_events(entity_type, entity_id);
			CREATE INDEX idx_entity_events_occurred_at ON entity_events(occurred_at DESC);
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
