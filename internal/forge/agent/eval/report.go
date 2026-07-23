package eval

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"
)

// WriteJSONReport writes the run result as JSON to the given writer.
func WriteJSONReport(w io.Writer, result *RunResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// WriteHTMLReport generates an HTML report file.
func WriteHTMLReport(path string, result *RunResult) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer f.Close()

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"statusIcon": func(passed bool) string {
			if passed {
				return "PASS"
			}
			return "FAIL"
		},
		"statusClass": func(passed bool) string {
			if passed {
				return "pass"
			}
			return "fail"
		},
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"pct": func(f float64) string {
			return fmt.Sprintf("%.0f%%", f*100)
		},
	}).Parse(htmlReportTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	return tmpl.Execute(f, result)
}

// FormatTextReport writes a human-readable text summary to the writer.
func FormatTextReport(w io.Writer, result *RunResult) {
	fmt.Fprintf(w, "Suite: %s  Agent: %s\n", result.SuiteName, result.AgentName)
	fmt.Fprintf(w, "Status: %s  Passed: %d  Failed: %d  Duration: %s\n\n",
		strings.ToUpper(result.Status), result.Passed, result.Failed,
		result.Duration.Round(time.Millisecond))

	for _, sr := range result.Scenarios {
		icon := "PASS"
		if !sr.Passed {
			icon = "FAIL"
		}
		fmt.Fprintf(w, "  [%s] %s (%s)\n", icon, sr.ScenarioName, sr.Duration.Round(time.Millisecond))

		if sr.Error != "" {
			fmt.Fprintf(w, "    Error: %s\n", sr.Error)
		}

		for _, a := range sr.Assertions {
			aIcon := "+"
			if !a.Passed {
				aIcon = "-"
			}
			fmt.Fprintf(w, "    %s %s: %s\n", aIcon, a.Type, a.Message)
		}
		fmt.Fprintln(w)
	}

	// Aggregated results
	if len(result.Aggregated) > 0 {
		fmt.Fprintln(w, "Aggregated Results:")
		for _, agg := range result.Aggregated {
			fmt.Fprintf(w, "  %s: %.0f%% pass rate (%d runs)\n",
				agg.ScenarioName, agg.PassRate*100, agg.TotalRuns)
			for _, a := range agg.Assertions {
				fmt.Fprintf(w, "    %s: %.0f%% pass rate\n", a.Type, a.PassRate*100)
				for _, f := range a.Failures {
					fmt.Fprintf(w, "      - %s\n", f)
				}
			}
		}
	}
}

const htmlReportTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Agent Eval Report — {{.SuiteName}}</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; color: #1a1a2e; }
  h1 { color: #7C3AED; }
  .summary { background: #f8f9fa; border-radius: 8px; padding: 1rem; margin-bottom: 2rem; }
  .pass { color: #10B981; font-weight: bold; }
  .fail { color: #EF4444; font-weight: bold; }
  .scenario { border: 1px solid #e2e8f0; border-radius: 8px; padding: 1rem; margin-bottom: 1rem; }
  .scenario h3 { margin-top: 0; }
  .assertion { padding: 0.25rem 0; font-size: 0.9rem; }
  .assertion .label { font-weight: 600; }
  table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #e2e8f0; }
  th { background: #f1f5f9; font-size: 0.85rem; text-transform: uppercase; }
</style>
</head>
<body>
<h1>Agent Eval Report</h1>
<div class="summary">
  <p><strong>Suite:</strong> {{.SuiteName}} &nbsp; <strong>Agent:</strong> {{.AgentName}}</p>
  <p><strong>Status:</strong> <span class="{{statusClass (eq .Status "passed")}}">{{.Status}}</span>
     &nbsp; Passed: {{.Passed}} &nbsp; Failed: {{.Failed}}
     &nbsp; Duration: {{formatDuration .Duration}}</p>
</div>

{{range .Scenarios}}
<div class="scenario">
  <h3><span class="{{statusClass .Passed}}">{{statusIcon .Passed}}</span> {{.ScenarioName}}</h3>
  <p>Duration: {{formatDuration .Duration}}</p>
  {{if .Error}}<p class="fail">Error: {{.Error}}</p>{{end}}
  {{range .Assertions}}
  <div class="assertion">
    <span class="{{statusClass .Passed}} label">{{statusIcon .Passed}}</span>
    <span class="label">{{.Type}}:</span> {{.Message}}
  </div>
  {{end}}
</div>
{{end}}

{{if .Aggregated}}
<h2>Aggregated Results</h2>
<table>
  <tr><th>Scenario</th><th>Runs</th><th>Pass Rate</th></tr>
  {{range .Aggregated}}
  <tr>
    <td>{{.ScenarioName}}</td>
    <td>{{.TotalRuns}}</td>
    <td>{{pct .PassRate}}</td>
  </tr>
  {{end}}
</table>
{{end}}

</body>
</html>
`
