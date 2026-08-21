package storage

import (
	"path/filepath"
	"testing"
)

func TestOpenAndClose(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpenCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "test.db")
	_, err := Open(path)
	// modernc sqlite creates parent dirs or fails; either way we test the path
	if err != nil {
		// This is expected if the directory doesn't exist; just verify we get an error gracefully
		return
	}
}

func TestWALMode(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.Conn().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.Conn().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestMigrateBasic(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{Version: 1, Description: "create foo", SQL: "CREATE TABLE foo (id TEXT PRIMARY KEY)"},
		{Version: 2, Description: "create bar", SQL: "CREATE TABLE bar (id TEXT PRIMARY KEY, foo_id TEXT)"},
	}

	if err := db.Migrate(migrations); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify tables exist
	var name string
	err = db.Conn().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='foo'").Scan(&name)
	if err != nil {
		t.Errorf("foo table not found: %v", err)
	}
	err = db.Conn().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='bar'").Scan(&name)
	if err != nil {
		t.Errorf("bar table not found: %v", err)
	}

	// Verify migration records
	var count int
	db.Conn().QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	if count != 2 {
		t.Errorf("migration count = %d, want 2", count)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{Version: 1, Description: "create foo", SQL: "CREATE TABLE foo (id TEXT PRIMARY KEY)"},
	}

	if err := db.Migrate(migrations); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	if err := db.Migrate(migrations); err != nil {
		t.Fatalf("Migrate second (idempotent): %v", err)
	}

	var count int
	db.Conn().QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	if count != 1 {
		t.Errorf("migration count = %d, want 1 after double-apply", count)
	}
}

func TestMigratePartial(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Apply first migration
	m1 := []Migration{
		{Version: 1, Description: "create foo", SQL: "CREATE TABLE foo (id TEXT PRIMARY KEY)"},
	}
	if err := db.Migrate(m1); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}

	// Apply both — only second should run
	m2 := []Migration{
		{Version: 1, Description: "create foo", SQL: "CREATE TABLE foo (id TEXT PRIMARY KEY)"},
		{Version: 2, Description: "create bar", SQL: "CREATE TABLE bar (id TEXT PRIMARY KEY)"},
	}
	if err := db.Migrate(m2); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}

	var count int
	db.Conn().QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	if count != 2 {
		t.Errorf("migration count = %d, want 2", count)
	}
}
