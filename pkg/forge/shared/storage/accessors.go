package storage

import "database/sql"

// DB returns the underlying *sql.DB for direct queries (used by analytics).
func (s *EvalStore) DB() *sql.DB {
	return s.db.Conn()
}

// DB returns the underlying *sql.DB for direct queries (used by analytics).
func (s *SessionStore) DB() *sql.DB {
	return s.db.Conn()
}
