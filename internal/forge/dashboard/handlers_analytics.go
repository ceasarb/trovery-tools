package dashboard

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type dataPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type latencyPoint struct {
	Date string  `json:"date"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
}

type usagePoint struct {
	ToolName string `json:"tool_name"`
	Count    int    `json:"count"`
}

// handleAnalyticsCost aggregates cost per day from session data.
func (s *Server) handleAnalyticsCost(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.sessStore.DB().Query(
		`SELECT date(substr(started_at, 1, 19)) AS day, SUM(total_cost_usd) AS cost
		 FROM sessions WHERE date(substr(started_at, 1, 19)) >= ? GROUP BY day ORDER BY day`,
		since,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics cost: "+err.Error())
		return
	}
	defer rows.Close()

	points := scanDataPoints(rows)
	writeJSON(w, http.StatusOK, listResponse{Data: points, Total: len(points)})
}

// handleAnalyticsTokens aggregates token usage per day from session data.
func (s *Server) handleAnalyticsTokens(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.sessStore.DB().Query(
		`SELECT date(substr(started_at, 1, 19)) AS day, SUM(total_tokens_in + total_tokens_out) AS tokens
		 FROM sessions WHERE date(substr(started_at, 1, 19)) >= ? GROUP BY day ORDER BY day`,
		since,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics tokens: "+err.Error())
		return
	}
	defer rows.Close()

	points := scanDataPoints(rows)
	writeJSON(w, http.StatusOK, listResponse{Data: points, Total: len(points)})
}

// handleAnalyticsErrors aggregates error counts per day from eval results.
func (s *Server) handleAnalyticsErrors(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.evalStore.DB().Query(
		`SELECT date(substr(er.created_at, 1, 19)) AS day, COUNT(*) AS errors
		 FROM eval_results er
		 WHERE er.status IN ('failed', 'error') AND date(substr(er.created_at, 1, 19)) >= ?
		 GROUP BY day ORDER BY day`,
		since,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics errors: "+err.Error())
		return
	}
	defer rows.Close()

	points := scanDataPoints(rows)
	writeJSON(w, http.StatusOK, listResponse{Data: points, Total: len(points)})
}

// handleAnalyticsLatency returns latency percentiles per day from eval results.
func (s *Server) handleAnalyticsLatency(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// SQLite doesn't have native percentile functions, so we compute per-day
	// using subqueries with NTILE.
	query := `
		WITH ranked AS (
			SELECT date(substr(created_at, 1, 19)) AS day, duration_ms,
				NTILE(100) OVER (PARTITION BY date(substr(created_at, 1, 19)) ORDER BY duration_ms) AS pct
			FROM eval_results
			WHERE duration_ms IS NOT NULL AND date(substr(created_at, 1, 19)) >= ?
		)
		SELECT day,
			MAX(CASE WHEN pct <= 50 THEN duration_ms END) AS p50,
			MAX(CASE WHEN pct <= 95 THEN duration_ms END) AS p95,
			MAX(CASE WHEN pct <= 99 THEN duration_ms END) AS p99
		FROM ranked
		GROUP BY day ORDER BY day`

	rows, err := s.evalStore.DB().Query(query, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics latency: "+err.Error())
		return
	}
	defer rows.Close()

	var points []latencyPoint
	for rows.Next() {
		var p latencyPoint
		if err := rows.Scan(&p.Date, &p.P50, &p.P95, &p.P99); err != nil {
			continue
		}
		points = append(points, p)
	}
	if points == nil {
		points = []latencyPoint{}
	}

	writeJSON(w, http.StatusOK, listResponse{Data: points, Total: len(points)})
}

// handleAnalyticsUsage returns tool usage frequency from session_tool_calls.
func (s *Server) handleAnalyticsUsage(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.sessStore.DB().Query(
		`SELECT tool_name, COUNT(*) AS cnt
		 FROM session_tool_calls WHERE date(substr(created_at, 1, 19)) >= ?
		 GROUP BY tool_name ORDER BY cnt DESC LIMIT 50`,
		since,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics usage: "+err.Error())
		return
	}
	defer rows.Close()

	var points []usagePoint
	for rows.Next() {
		var p usagePoint
		if err := rows.Scan(&p.ToolName, &p.Count); err != nil {
			continue
		}
		points = append(points, p)
	}
	if points == nil {
		points = []usagePoint{}
	}

	writeJSON(w, http.StatusOK, listResponse{Data: points, Total: len(points)})
}

func scanDataPoints(rows *sql.Rows) []dataPoint {
	var points []dataPoint
	for rows.Next() {
		var p dataPoint
		if err := rows.Scan(&p.Date, &p.Value); err != nil {
			fmt.Println("scan data point:", err)
			continue
		}
		points = append(points, p)
	}
	if points == nil {
		points = []dataPoint{}
	}
	return points
}
