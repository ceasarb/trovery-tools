package deploy

import "fmt"

type observabilityData struct {
	AgentName string
	Port      int
}

func (d *Deployer) buildObsData() observabilityData {
	return observabilityData{
		AgentName: d.config.Name,
		Port:      8080,
	}
}

const grafanaDashboardTmpl = `{
  "annotations": { "list": [] },
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 0,
  "id": null,
  "links": [],
  "panels": [
    {
      "title": "Request Rate",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 0 },
      "targets": [
        {
          "expr": "rate(demi_requests_total[5m])",
          "legendFormat": "{{ "{{" }}endpoint{{ "}}" }} {{ "{{" }}status{{ "}}" }}"
        }
      ]
    },
    {
      "title": "Request Duration (p95)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 0 },
      "targets": [
        {
          "expr": "histogram_quantile(0.95, rate(demi_request_duration_seconds_bucket[5m]))",
          "legendFormat": "p95 {{ "{{" }}endpoint{{ "}}" }}"
        }
      ]
    },
    {
      "title": "Active Sessions",
      "type": "gauge",
      "gridPos": { "h": 4, "w": 6, "x": 0, "y": 8 },
      "targets": [
        { "expr": "demi_active_sessions" }
      ]
    },
    {
      "title": "Budget Remaining",
      "type": "gauge",
      "gridPos": { "h": 4, "w": 6, "x": 6, "y": 8 },
      "targets": [
        { "expr": "demi_budget_remaining_usd" }
      ]
    },
    {
      "title": "Total Cost (USD)",
      "type": "stat",
      "gridPos": { "h": 4, "w": 6, "x": 12, "y": 8 },
      "targets": [
        { "expr": "demi_cost_usd_total" }
      ]
    },
    {
      "title": "Tool Calls / Errors",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 12 },
      "targets": [
        { "expr": "rate(demi_tool_calls_total[5m])", "legendFormat": "calls" },
        { "expr": "rate(demi_tool_call_errors_total[5m])", "legendFormat": "errors" }
      ]
    },
    {
      "title": "Token Throughput",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 12 },
      "targets": [
        { "expr": "rate(demi_tokens_input_total[5m])", "legendFormat": "input" },
        { "expr": "rate(demi_tokens_output_total[5m])", "legendFormat": "output" }
      ]
    },
    {
      "title": "Tool Call Duration (p95)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 20 },
      "targets": [
        {
          "expr": "histogram_quantile(0.95, rate(demi_tool_call_duration_seconds_bucket[5m]))",
          "legendFormat": "p95"
        }
      ]
    },
    {
      "title": "Cost Rate (USD/min)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 20 },
      "targets": [
        { "expr": "rate(demi_cost_usd_total[5m]) * 60", "legendFormat": "$/min" }
      ]
    }
  ],
  "schemaVersion": 39,
  "tags": ["demigo", "agent"],
  "templating": { "list": [] },
  "time": { "from": "now-1h", "to": "now" },
  "title": "{{ .AgentName }} — Agent Dashboard",
  "uid": "{{ .AgentName }}-dashboard"
}
`

const prometheusAlertRulesTmpl = `# {{ .AgentName }} — Prometheus Alert Rules
# Import into Prometheus via rule_files config.

groups:
  - name: {{ .AgentName }}_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(demi_tool_call_errors_total[5m]) / rate(demi_tool_calls_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
          agent: {{ .AgentName }}
        annotations:
          summary: "High tool call error rate (> 10%)"
          description: "Agent {{ .AgentName }} tool call error rate is above 10% for 5 minutes."

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(demi_request_duration_seconds_bucket[5m])) > 30
        for: 5m
        labels:
          severity: warning
          agent: {{ .AgentName }}
        annotations:
          summary: "High request latency (p95 > 30s)"
          description: "Agent {{ .AgentName }} p95 latency is above 30 seconds."

      - alert: BudgetLow
        expr: demi_budget_remaining_usd < 10
        for: 1m
        labels:
          severity: critical
          agent: {{ .AgentName }}
        annotations:
          summary: "Monthly budget nearly exhausted (< $10 remaining)"
          description: "Agent {{ .AgentName }} has less than $10 remaining in monthly budget."

      - alert: NoActiveRequests
        expr: rate(demi_requests_total[15m]) == 0
        for: 15m
        labels:
          severity: info
          agent: {{ .AgentName }}
        annotations:
          summary: "No requests in 15 minutes"
          description: "Agent {{ .AgentName }} has not received any requests in 15 minutes."
`

const gcpAlertPolicyTmpl = `# {{ .AgentName }} — GCP Cloud Monitoring Alert Policies
# Apply via: gcloud alpha monitoring policies create --policy-from-file=gcp-alerts.yaml

# Note: These use Cloud Run metrics. Custom metrics from /metrics require
# a Prometheus-to-Cloud-Monitoring bridge (e.g., google-cloud-prometheus).
#
# For custom demi_* metrics, deploy a Prometheus server with remote_write
# to Cloud Monitoring, or use the GMP (Google Managed Prometheus) service.

displayName: "{{ .AgentName }} — High Error Rate"
combiner: OR
conditions:
  - displayName: "Error rate > 10%"
    conditionThreshold:
      filter: 'resource.type="cloud_run_revision" AND metric.type="run.googleapis.com/request_count" AND metric.labels.response_code_class="5xx"'
      comparison: COMPARISON_GT
      thresholdValue: 0.1
      duration: 300s
      aggregations:
        - alignmentPeriod: 60s
          perSeriesAligner: ALIGN_RATE
`

const azureAlertTmpl = `// {{ .AgentName }} — Azure Monitor Alert Rules (Bicep)
// Deploy alongside main.bicep or separately.

@description('Container App resource ID')
param containerAppId string

@description('Action group resource ID for notifications')
param actionGroupId string

resource highLatencyAlert 'Microsoft.Insights/metricAlerts@2018-03-01' = {
  name: '{{ .AgentName }}-high-latency'
  location: 'global'
  properties: {
    severity: 2
    scopes: [containerAppId]
    evaluationFrequency: 'PT5M'
    windowSize: 'PT15M'
    criteria: {
      'odata.type': 'Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria'
      allOf: [
        {
          name: 'HighLatency'
          metricName: 'RequestDuration'
          operator: 'GreaterThan'
          threshold: 30000
          timeAggregation: 'Average'
        }
      ]
    }
    actions: [
      { actionGroupId: actionGroupId }
    ]
  }
}
`

// GenerateObservabilityArtifacts generates Grafana dashboard and alert rules
// and writes them to the deployer's output directory.
func (d *Deployer) GenerateObservabilityArtifacts() ([]DeployedFile, error) {
	data := d.buildObsData()

	templates := []struct {
		path string
		tmpl string
		desc string
	}{
		{"observability/grafana-dashboard.json", grafanaDashboardTmpl, "Grafana dashboard (import via UI or API)"},
		{"observability/prometheus-alerts.yml", prometheusAlertRulesTmpl, "Prometheus alert rules"},
		{"observability/gcp-alerts.yaml", gcpAlertPolicyTmpl, "GCP Cloud Monitoring alert policy"},
		{"observability/azure-alerts.bicep", azureAlertTmpl, "Azure Monitor alert rules (Bicep)"},
	}

	var files []DeployedFile
	for _, t := range templates {
		content, err := renderTemplate(t.path, t.tmpl, data)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", t.path, err)
		}
		if err := d.writeFile(t.path, content); err != nil {
			return nil, err
		}
		files = append(files, DeployedFile{Path: t.path, Description: t.desc})
	}

	return files, nil
}
