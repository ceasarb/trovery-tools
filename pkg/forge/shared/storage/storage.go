package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Migration represents a single schema migration.
type Migration struct {
	Version     int
	Description string
	SQL         string
}

// DB wraps a sql.DB connection with SQLite-specific configuration.
type DB struct {
	conn *sql.DB
}

// Open creates or opens a SQLite database at path with WAL mode, busy timeout,
// and foreign keys enabled.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("set %s: %w", p, err)
		}
	}

	return &DB{conn: conn}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for direct queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Migrate applies pending migrations in order. Migrations are tracked in a
// _migrations table. Already-applied versions are skipped.
func (db *DB) Migrate(migrations []Migration) error {
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		version     INTEGER PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	for _, m := range migrations {
		var count int
		err := db.conn.QueryRow("SELECT COUNT(*) FROM _migrations WHERE version = ?", m.Version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if count > 0 {
			continue
		}

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Description, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO _migrations (version, description) VALUES (?, ?)",
			m.Version, m.Description,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}
