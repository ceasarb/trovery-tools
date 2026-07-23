package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const maxContentBytes = 50 * 1024 // 50KB

// Session represents an agent chat session.
type Session struct {
	ID            string
	AgentName     string
	Provider      string
	Model         string
	StartedAt     time.Time
	FinishedAt    *time.Time
	TotalTurns    int
	TotalTokensIn int
	TotalTokensOut int
	TotalCostUSD  float64
	Summary       *string
}

// SessionTurn represents a single turn in a session.
type SessionTurn struct {
	ID        string
	SessionID string
	TurnNumber int
	Role       string // user, assistant
	Content    string
	TokensIn   int
	TokensOut  int
	CostUSD    float64
	CreatedAt  time.Time
}

// SessionToolCall represents a tool invocation within a turn.
type SessionToolCall struct {
	ID            string
	TurnID        string
	SessionID     string
	ToolName      string
	ServerName    string
	ArgumentsJSON *string
	ResultJSON    *string
	DurationMs    *int64
	Error         *string
	CreatedAt     time.Time
}

var sessionMigrations = []Migration{
	{
		Version:     100,
		Description: "create sessions table",
		SQL: `CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_name TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			total_turns INTEGER DEFAULT 0,
			total_tokens_in INTEGER DEFAULT 0,
			total_tokens_out INTEGER DEFAULT 0,
			total_cost_usd REAL DEFAULT 0,
			summary TEXT
		)`,
	},
	{
		Version:     101,
		Description: "create session_turns table",
		SQL: `CREATE TABLE session_turns (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id),
			turn_number INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			tokens_in INTEGER DEFAULT 0,
			tokens_out INTEGER DEFAULT 0,
			cost_usd REAL DEFAULT 0,
			created_at DATETIME NOT NULL
		)`,
	},
	{
		Version:     102,
		Description: "create session_tool_calls table",
		SQL: `CREATE TABLE session_tool_calls (
			id TEXT PRIMARY KEY,
			turn_id TEXT NOT NULL REFERENCES session_turns(id),
			session_id TEXT NOT NULL REFERENCES sessions(id),
			tool_name TEXT NOT NULL,
			server_name TEXT NOT NULL,
			arguments_json TEXT,
			result_json TEXT,
			duration_ms INTEGER,
			error TEXT,
			created_at DATETIME NOT NULL
		)`,
	},
	{
		Version:     103,
		Description: "add indexes for session queries",
		SQL: `CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_name);
			  CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at);
			  CREATE INDEX IF NOT EXISTS idx_session_turns_session ON session_turns(session_id);
			  CREATE INDEX IF NOT EXISTS idx_session_tool_calls_session ON session_tool_calls(session_id);
			  CREATE INDEX IF NOT EXISTS idx_session_tool_calls_turn ON session_tool_calls(turn_id)`,
	},
}

// SessionStore provides CRUD operations for agent sessions, turns, and tool calls.
type SessionStore struct {
	db *DB
}

// NewSessionStore opens a SQLite database at dbPath and runs session migrations.
func NewSessionStore(dbPath string) (*SessionStore, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(sessionMigrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("session migrations: %w", err)
	}
	return &SessionStore{db: db}, nil
}

// Close closes the underlying database.
func (s *SessionStore) Close() error {
	return s.db.Close()
}

// truncateContent truncates content to maxContentBytes, appending a marker if truncated.
func truncateContent(content string) string {
	if len(content) <= maxContentBytes {
		return content
	}
	return content[:maxContentBytes] + "\n[truncated]"
}

// CreateSession inserts a new session. If ID is empty, a UUID is generated.
func (s *SessionStore) CreateSession(session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	_, err := s.db.Conn().Exec(
		`INSERT INTO sessions (id, agent_name, provider, model, started_at, finished_at,
		 total_turns, total_tokens_in, total_tokens_out, total_cost_usd, summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.AgentName, session.Provider, session.Model,
		session.StartedAt, session.FinishedAt, session.TotalTurns,
		session.TotalTokensIn, session.TotalTokensOut, session.TotalCostUSD, session.Summary,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// UpdateSession updates an existing session.
func (s *SessionStore) UpdateSession(session *Session) error {
	_, err := s.db.Conn().Exec(
		`UPDATE sessions SET agent_name=?, provider=?, model=?, started_at=?, finished_at=?,
		 total_turns=?, total_tokens_in=?, total_tokens_out=?, total_cost_usd=?, summary=? WHERE id=?`,
		session.AgentName, session.Provider, session.Model,
		session.StartedAt, session.FinishedAt, session.TotalTurns,
		session.TotalTokensIn, session.TotalTokensOut, session.TotalCostUSD, session.Summary,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (s *SessionStore) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.Conn().QueryRow(
		`SELECT id, agent_name, provider, model, started_at, finished_at,
		 total_turns, total_tokens_in, total_tokens_out, total_cost_usd, summary
		 FROM sessions WHERE id=?`, id,
	).Scan(
		&sess.ID, &sess.AgentName, &sess.Provider, &sess.Model,
		&sess.StartedAt, &sess.FinishedAt, &sess.TotalTurns,
		&sess.TotalTokensIn, &sess.TotalTokensOut, &sess.TotalCostUSD, &sess.Summary,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &sess, nil
}

// ListSessions returns sessions ordered by started_at descending.
// If agentName is non-empty, results are filtered. Supports pagination via limit/offset.
func (s *SessionStore) ListSessions(agentName string, limit, offset int) ([]Session, error) {
	var rows *sql.Rows
	var err error

	if agentName != "" {
		rows, err = s.db.Conn().Query(
			`SELECT id, agent_name, provider, model, started_at, finished_at,
			 total_turns, total_tokens_in, total_tokens_out, total_cost_usd, summary
			 FROM sessions WHERE agent_name=? ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			agentName, limit, offset,
		)
	} else {
		rows, err = s.db.Conn().Query(
			`SELECT id, agent_name, provider, model, started_at, finished_at,
			 total_turns, total_tokens_in, total_tokens_out, total_cost_usd, summary
			 FROM sessions ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(
			&sess.ID, &sess.AgentName, &sess.Provider, &sess.Model,
			&sess.StartedAt, &sess.FinishedAt, &sess.TotalTurns,
			&sess.TotalTokensIn, &sess.TotalTokensOut, &sess.TotalCostUSD, &sess.Summary,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// CreateTurn inserts a new session turn. Content is truncated to 50KB.
// If ID is empty, a UUID is generated.
func (s *SessionStore) CreateTurn(turn *SessionTurn) error {
	if turn.ID == "" {
		turn.ID = uuid.New().String()
	}
	turn.Content = truncateContent(turn.Content)
	_, err := s.db.Conn().Exec(
		`INSERT INTO session_turns (id, session_id, turn_number, role, content, tokens_in, tokens_out, cost_usd, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		turn.ID, turn.SessionID, turn.TurnNumber, turn.Role, turn.Content,
		turn.TokensIn, turn.TokensOut, turn.CostUSD, turn.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session_turn: %w", err)
	}
	return nil
}

// GetTurnsBySession returns all turns for a session ordered by turn number.
func (s *SessionStore) GetTurnsBySession(sessionID string) ([]SessionTurn, error) {
	rows, err := s.db.Conn().Query(
		`SELECT id, session_id, turn_number, role, content, tokens_in, tokens_out, cost_usd, created_at
		 FROM session_turns WHERE session_id=? ORDER BY turn_number`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list session_turns: %w", err)
	}
	defer rows.Close()

	var turns []SessionTurn
	for rows.Next() {
		var t SessionTurn
		if err := rows.Scan(
			&t.ID, &t.SessionID, &t.TurnNumber, &t.Role, &t.Content,
			&t.TokensIn, &t.TokensOut, &t.CostUSD, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session_turn: %w", err)
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

// CreateToolCall inserts a new tool call record. If ID is empty, a UUID is generated.
func (s *SessionStore) CreateToolCall(call *SessionToolCall) error {
	if call.ID == "" {
		call.ID = uuid.New().String()
	}
	_, err := s.db.Conn().Exec(
		`INSERT INTO session_tool_calls (id, turn_id, session_id, tool_name, server_name, arguments_json, result_json, duration_ms, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		call.ID, call.TurnID, call.SessionID, call.ToolName, call.ServerName,
		call.ArgumentsJSON, call.ResultJSON, call.DurationMs, call.Error, call.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session_tool_call: %w", err)
	}
	return nil
}

// GetToolCallsByTurn returns all tool calls for a given turn.
func (s *SessionStore) GetToolCallsByTurn(turnID string) ([]SessionToolCall, error) {
	rows, err := s.db.Conn().Query(
		`SELECT id, turn_id, session_id, tool_name, server_name, arguments_json, result_json, duration_ms, error, created_at
		 FROM session_tool_calls WHERE turn_id=? ORDER BY created_at`,
		turnID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tool_calls by turn: %w", err)
	}
	defer rows.Close()

	return scanToolCalls(rows)
}

// GetToolCallsBySession returns all tool calls for a given session.
func (s *SessionStore) GetToolCallsBySession(sessionID string) ([]SessionToolCall, error) {
	rows, err := s.db.Conn().Query(
		`SELECT id, turn_id, session_id, tool_name, server_name, arguments_json, result_json, duration_ms, error, created_at
		 FROM session_tool_calls WHERE session_id=? ORDER BY created_at`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tool_calls by session: %w", err)
	}
	defer rows.Close()

	return scanToolCalls(rows)
}

// PruneSessions deletes sessions older than maxAge and their associated turns
// and tool calls. Returns the number of sessions deleted.
func (s *SessionStore) PruneSessions(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)

	tx, err := s.db.Conn().Begin()
	if err != nil {
		return 0, fmt.Errorf("begin prune tx: %w", err)
	}

	// Delete tool calls for old sessions
	if _, err := tx.Exec(
		"DELETE FROM session_tool_calls WHERE session_id IN (SELECT id FROM sessions WHERE started_at < ?)",
		cutoff,
	); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("prune tool_calls: %w", err)
	}

	// Delete turns for old sessions
	if _, err := tx.Exec(
		"DELETE FROM session_turns WHERE session_id IN (SELECT id FROM sessions WHERE started_at < ?)",
		cutoff,
	); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("prune turns: %w", err)
	}

	// Delete old sessions
	result, err := tx.Exec("DELETE FROM sessions WHERE started_at < ?", cutoff)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("prune sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune: %w", err)
	}

	return int(count), nil
}

func scanToolCalls(rows *sql.Rows) ([]SessionToolCall, error) {
	var calls []SessionToolCall
	for rows.Next() {
		var c SessionToolCall
		if err := rows.Scan(
			&c.ID, &c.TurnID, &c.SessionID, &c.ToolName, &c.ServerName,
			&c.ArgumentsJSON, &c.ResultJSON, &c.DurationMs, &c.Error, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tool_call: %w", err)
		}
		calls = append(calls, c)
	}
	return calls, rows.Err()
}
