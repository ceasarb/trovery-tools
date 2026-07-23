package session

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	status TEXT,
	start_time TEXT,
	end_time TEXT,
	duration_seconds REAL,
	tool_names TEXT,
	file_changes_count INTEGER,
	violations_count INTEGER,
	error_violations_count INTEGER,
	total_tokens INTEGER,
	total_cost_usd REAL,
	config_hash TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
`

// SQLiteIndex provides queryable session indexing backed by SQLite.
type SQLiteIndex struct {
	db *sql.DB
}

// OpenSQLiteIndex opens (or creates) the SQLite database at the given path.
func OpenSQLiteIndex(dbPath string) (*SQLiteIndex, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	// Enable WAL mode for concurrent reads and better write performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return &SQLiteIndex{db: db}, nil
}

// Close closes the database connection.
func (idx *SQLiteIndex) Close() error {
	return idx.db.Close()
}

// Index upserts a session into the SQLite index.
func (idx *SQLiteIndex) Index(s *Session) error {
	toolNames := extractToolNames(s)
	errCount := countErrors(s)

	var totalTokens int
	var totalCost float64
	for _, u := range s.TokenUsage {
		totalTokens += u.TotalTokens
		totalCost += u.EstimatedCostUSD
	}

	_, err := idx.db.Exec(`
		INSERT OR REPLACE INTO sessions
			(id, status, start_time, end_time, duration_seconds, tool_names,
			 file_changes_count, violations_count, error_violations_count,
			 total_tokens, total_cost_usd, config_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, string(s.Status),
		s.StartTime.Format(time.RFC3339),
		formatEndTime(s.EndTime),
		s.DurationSeconds,
		toolNames,
		len(s.FileChanges),
		len(s.Violations),
		errCount,
		totalTokens,
		totalCost,
		s.ConfigHash,
	)
	return err
}

// QueryOpts defines filters for querying sessions.
type QueryOpts struct {
	Tool          string
	Since         time.Time
	HasViolations bool
	Status        string
	Limit         int
}

// Query returns sessions matching the given filters.
// Returns only indexed metadata — load full JSON for details.
func (idx *SQLiteIndex) Query(opts QueryOpts) ([]SessionSummary, error) {
	var conditions []string
	var args []interface{}

	if opts.Tool != "" {
		conditions = append(conditions, "tool_names LIKE ?")
		args = append(args, "%"+opts.Tool+"%")
	}
	if !opts.Since.IsZero() {
		conditions = append(conditions, "start_time >= ?")
		args = append(args, opts.Since.Format(time.RFC3339))
	}
	if opts.HasViolations {
		conditions = append(conditions, "violations_count > 0")
	}
	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, opts.Status)
	}

	query := "SELECT id, status, start_time, end_time, duration_seconds, tool_names, file_changes_count, violations_count, error_violations_count, total_tokens, total_cost_usd FROM sessions"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY start_time DESC"

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SessionSummary
	for rows.Next() {
		var s SessionSummary
		var startStr, endStr string
		if err := rows.Scan(&s.ID, &s.Status, &startStr, &endStr, &s.DurationSeconds,
			&s.ToolNames, &s.FileChangesCount, &s.ViolationsCount, &s.ErrorViolationsCount,
			&s.TotalTokens, &s.TotalCostUSD); err != nil {
			return nil, err
		}
		s.StartTime, _ = time.Parse(time.RFC3339, startStr)
		if endStr != "" {
			s.EndTime, _ = time.Parse(time.RFC3339, endStr)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// AggregateStats holds aggregated session statistics.
type AggregateStats struct {
	Count          int
	TotalDuration  float64
	TotalFiles     int
	TotalViolations int
	TotalTokens    int
	TotalCost      float64
}

// Aggregate groups sessions by the given field and computes statistics.
func (idx *SQLiteIndex) Aggregate(groupBy string, since time.Time) (map[string]AggregateStats, error) {
	var groupCol string
	switch groupBy {
	case "tool":
		groupCol = "tool_names"
	case "status":
		groupCol = "status"
	default:
		groupCol = "status"
	}

	query := fmt.Sprintf(`
		SELECT %s,
			COUNT(*) as cnt,
			COALESCE(SUM(duration_seconds), 0),
			COALESCE(SUM(file_changes_count), 0),
			COALESCE(SUM(violations_count), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(total_cost_usd), 0)
		FROM sessions
		WHERE start_time >= ?
		GROUP BY %s`, groupCol, groupCol)

	rows, err := idx.db.Query(query, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]AggregateStats)
	for rows.Next() {
		var key string
		var stats AggregateStats
		if err := rows.Scan(&key, &stats.Count, &stats.TotalDuration,
			&stats.TotalFiles, &stats.TotalViolations,
			&stats.TotalTokens, &stats.TotalCost); err != nil {
			return nil, err
		}
		results[key] = stats
	}
	return results, rows.Err()
}

// RebuildIndex re-indexes all JSON session files into SQLite.
func (idx *SQLiteIndex) RebuildIndex(sessionsDir string) (int, error) {
	sessions, err := ListSessions(sessionsDir, 0)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, s := range sessions {
		if err := idx.Index(s); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// SessionSummary is a lightweight session record from the SQLite index.
type SessionSummary struct {
	ID                   string
	Status               string
	StartTime            time.Time
	EndTime              time.Time
	DurationSeconds      float64
	ToolNames            string
	FileChangesCount     int
	ViolationsCount      int
	ErrorViolationsCount int
	TotalTokens          int
	TotalCostUSD         float64
}

func extractToolNames(s *Session) string {
	seen := make(map[string]bool)
	var names []string
	for _, tr := range s.ToolRuns {
		if !seen[tr.Tool] {
			seen[tr.Tool] = true
			names = append(names, tr.Tool)
		}
	}
	return strings.Join(names, ",")
}

func countErrors(s *Session) int {
	count := 0
	for _, v := range s.Violations {
		if v.Severity == "error" {
			count++
		}
	}
	return count
}

func formatEndTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
