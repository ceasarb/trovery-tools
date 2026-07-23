package eval

import (
	"bytes"
	"html/template"
	"time"
)

// ReportData holds the data passed to the HTML report template.
type ReportData struct {
	SuiteName   string
	RunDate     string
	Total       int
	Passed      int
	Failed      int
	Skipped     int
	Status      string
	Scenarios   []ScenarioResult
	Regressions []Regression
}

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Eval Report: {{.SuiteName}}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f9fafb; color: #111827; padding: 2rem; }
  .container { max-width: 800px; margin: 0 auto; }
  h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
  .meta { color: #6b7280; font-size: 0.875rem; margin-bottom: 1.5rem; }
  .summary { display: flex; gap: 1rem; margin-bottom: 2rem; }
  .stat { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 1rem 1.5rem; flex: 1; text-align: center; }
  .stat .num { font-size: 2rem; font-weight: 700; }
  .stat .label { font-size: 0.75rem; color: #6b7280; text-transform: uppercase; }
  .stat.passed .num { color: #10b981; }
  .stat.failed .num { color: #ef4444; }
  .stat.skipped .num { color: #f59e0b; }
  .scenario { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 1rem 1.5rem; margin-bottom: 0.75rem; }
  .scenario-header { display: flex; justify-content: space-between; align-items: center; }
  .scenario-name { font-weight: 600; }
  .badge { padding: 0.125rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
  .badge.passed { background: #d1fae5; color: #065f46; }
  .badge.failed { background: #fee2e2; color: #991b1b; }
  .badge.error { background: #fef3c7; color: #92400e; }
  .assertions { margin-top: 0.75rem; font-size: 0.875rem; }
  .assertion { padding: 0.25rem 0; display: flex; gap: 0.5rem; }
  .assertion .icon { width: 1rem; }
  .assertion.pass .icon { color: #10b981; }
  .assertion.fail .icon { color: #ef4444; }
  .error-msg { color: #ef4444; font-size: 0.875rem; margin-top: 0.5rem; }
  .regressions { background: #fef2f2; border: 1px solid #fecaca; border-radius: 8px; padding: 1rem 1.5rem; margin-bottom: 2rem; }
  .regressions h2 { color: #991b1b; font-size: 1rem; margin-bottom: 0.5rem; }
  .regression-item { font-size: 0.875rem; padding: 0.25rem 0; }
</style>
</head>
<body>
<div class="container">
  <h1>{{.SuiteName}}</h1>
  <div class="meta">{{.RunDate}} &middot; Status: {{.Status}}</div>

  <div class="summary">
    <div class="stat passed"><div class="num">{{.Passed}}</div><div class="label">Passed</div></div>
    <div class="stat failed"><div class="num">{{.Failed}}</div><div class="label">Failed</div></div>
    <div class="stat skipped"><div class="num">{{.Skipped}}</div><div class="label">Skipped</div></div>
  </div>

  {{if .Regressions}}
  <div class="regressions">
    <h2>Regressions Detected</h2>
    {{range .Regressions}}
    <div class="regression-item">{{.ScenarioName}}: was {{.PreviousStatus}}, now {{.CurrentStatus}}</div>
    {{end}}
  </div>
  {{end}}

  {{range .Scenarios}}
  <div class="scenario">
    <div class="scenario-header">
      <span class="scenario-name">{{.Name}}</span>
      <span class="badge {{.Status}}">{{.Status}}</span>
    </div>
    {{if .Error}}
    <div class="error-msg">{{.Error}}</div>
    {{end}}
    {{if .Assertions}}
    <div class="assertions">
      {{range .Assertions}}
      <div class="assertion {{if .Passed}}pass{{else}}fail{{end}}">
        <span class="icon">{{if .Passed}}&#10003;{{else}}&#10007;{{end}}</span>
        <span>{{.Type}}({{.Field}}): {{.Message}}</span>
      </div>
      {{end}}
    </div>
    {{end}}
  </div>
  {{end}}
</div>
</body>
</html>`

// GenerateReport renders an HTML report for an eval run.
func GenerateReport(result *RunResult, regressions []Regression) (string, error) {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return "", err
	}

	data := ReportData{
		SuiteName:   result.SuiteName,
		RunDate:     time.Now().Format("2006-01-02 15:04:05"),
		Total:       result.Total,
		Passed:      result.Passed,
		Failed:      result.Failed,
		Skipped:     result.Skipped,
		Status:      result.Status,
		Scenarios:   result.Scenarios,
		Regressions: regressions,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
