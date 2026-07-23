package reporting

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

// ReportGenerator creates reports from session data.
type ReportGenerator struct {
	idx *session.SQLiteIndex
}

// NewReportGenerator creates a generator backed by a SQLite index.
func NewReportGenerator(idx *session.SQLiteIndex) *ReportGenerator {
	return &ReportGenerator{idx: idx}
}

// ReportOptions configures report generation.
type ReportOptions struct {
	Days    int
	GroupBy string // "tool" or "status"
	Format  string // "terminal", "json", "csv"
}

// ReportData holds the generated report content.
type ReportData struct {
	GeneratedAt time.Time                        `json:"generated_at"`
	Days        int                              `json:"days"`
	GroupBy     string                           `json:"group_by"`
	Groups      map[string]session.AggregateStats `json:"groups"`
}

// Generate produces a formatted report string.
func (g *ReportGenerator) Generate(opts ReportOptions) (string, error) {
	since := time.Now().AddDate(0, 0, -opts.Days)
	if opts.Days <= 0 {
		since = time.Time{} // All time.
	}

	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = "status"
	}

	stats, err := g.idx.Aggregate(groupBy, since)
	if err != nil {
		return "", fmt.Errorf("aggregating data: %w", err)
	}

	data := ReportData{
		GeneratedAt: time.Now(),
		Days:        opts.Days,
		GroupBy:     groupBy,
		Groups:      stats,
	}

	switch opts.Format {
	case "json":
		return formatJSON(data)
	case "csv":
		return formatCSV(data)
	default:
		return formatTerminal(data), nil
	}
}

func formatJSON(data ReportData) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatCSV(data ReportData) (string, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	w.Write([]string{"Group", "Sessions", "Duration(s)", "Files", "Violations", "Tokens", "Cost(USD)"})
	for group, stats := range data.Groups {
		w.Write([]string{
			group,
			fmt.Sprintf("%d", stats.Count),
			fmt.Sprintf("%.0f", stats.TotalDuration),
			fmt.Sprintf("%d", stats.TotalFiles),
			fmt.Sprintf("%d", stats.TotalViolations),
			fmt.Sprintf("%d", stats.TotalTokens),
			fmt.Sprintf("%.4f", stats.TotalCost),
		})
	}
	w.Flush()
	return buf.String(), w.Error()
}

func formatTerminal(data ReportData) string {
	var sb strings.Builder

	period := "all time"
	if data.Days > 0 {
		period = fmt.Sprintf("last %d days", data.Days)
	}
	sb.WriteString(fmt.Sprintf("Usage Report (%s, by %s)\n", period, data.GroupBy))
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	if len(data.Groups) == 0 {
		sb.WriteString("No sessions found.\n")
		return sb.String()
	}

	for group, stats := range data.Groups {
		sb.WriteString(fmt.Sprintf("\n  %s:\n", group))
		sb.WriteString(fmt.Sprintf("    Sessions:   %d\n", stats.Count))
		sb.WriteString(fmt.Sprintf("    Duration:   %.0fs\n", stats.TotalDuration))
		sb.WriteString(fmt.Sprintf("    Files:      %d\n", stats.TotalFiles))
		sb.WriteString(fmt.Sprintf("    Violations: %d\n", stats.TotalViolations))
		sb.WriteString(fmt.Sprintf("    Tokens:     %d\n", stats.TotalTokens))
		sb.WriteString(fmt.Sprintf("    Cost:       $%.4f\n", stats.TotalCost))
	}

	return sb.String()
}
