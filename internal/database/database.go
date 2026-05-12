package database

import (
	"database/sql"
	"fmt"

	"git.sr.ht/~jakintosh/compass/internal/service"
	_ "modernc.org/sqlite"
)

type Options struct {
	Path string
	WAL  bool
}

type DB struct {
	Conn *sql.DB
}

var _ service.Store = (*DB)(nil)

func Open(
	opts Options,
) (
	*DB,
	error,
) {
	const busyTimeoutMS = 5000

	conn, err := sql.Open("sqlite", opts.Path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := conn.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", busyTimeoutMS)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if opts.WAL {
		if _, err := conn.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			conn.Close()
			return nil, fmt.Errorf("enable wal mode: %w", err)
		}
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{Conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}
