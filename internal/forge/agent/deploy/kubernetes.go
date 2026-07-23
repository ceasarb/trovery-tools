package deploy

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

type helmData struct {
	AgentName       string
	ChartName       string
	Provider        string
	APIKeyEnv       string
	Port            int
	BundledServers  []config.ServerRef
	ExternalServers []config.ServerRef
	HasBundled      bool
	MaxTokens       int
	Model           string
}

func (d *Deployer) buildHelmData() helmData {
	bundled, external := d.classifyServers()
	return helmData{
		AgentName:       d.config.Name,
		ChartName:       d.config.Name,
		Provider:        d.config.Model.Provider,
		APIKeyEnv:       d.apiKeyEnv(),
		Port:            8080,
		BundledServers:  bundled,
		ExternalServers: external,
		HasBundled:      len(bundled) > 0,
		MaxTokens:       d.config.Model.MaxTokens,
		Model:           d.config.Model.Model,
	}
}

const helmChartYamlTmpl = `apiVersion: v2
name: {{ .ChartName }}
description: Helm chart for {{ .AgentName }} agent (Demigo Forge)
type: application
version: 0.1.0
appVersion: "1.0.0"
`

// Use raw delimiters to avoid Go template engine interpreting Helm mustache.
// We write Helm templates as literal strings (no Go template interpolation).
const helmValuesTmpl = `# {{ .AgentName }} — Helm values
replicaCount: 1

image:
  repository: {{ .AgentName }}
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: {{ .Port }}

agent:
  provider: {{ .Provider }}
  model: {{ .Model }}
  port: {{ .Port }}

env:
  {{ .APIKeyEnv }}: ""

resources:
  limits:
    cpu: "1"
    memory: 256Mi
  requests:
    cpu: 250m
    memory: 128Mi

autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 70

networkPolicy:
  enabled: true

serviceMonitor:
  enabled: false
  interval: 30s
{{ if .HasBundled }}
sidecars:
{{ range .BundledServers }}  - name: {{ .Name }}
    image: {{ .Name }}:latest
    resources:
      limits:
        cpu: 500m
        memory: 128Mi
      requests:
        cpu: 100m
        memory: 64Mi
{{ end }}{{ end }}`

func (d *Deployer) helmDeploymentYaml() string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "chart.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "chart.selectorLabels" . | nindent 8 }}
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
      containers:
        - name: agent
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.agent.port }}
              protocol: TCP
          env:
            - name: {{ .Values.env | keys | first }}
              valueFrom:
                secretKeyRef:
                  name: {{ include "chart.fullname" . }}-secret
                  key: api-key
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
        {{- range .Values.sidecars }}
        - name: {{ .name }}
          image: "{{ .image }}"
          resources:
            {{- toYaml .resources | nindent 12 }}
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
        {{- end }}
`
}

func (d *Deployer) helmServiceYaml() string {
	return `apiVersion: v1
kind: Service
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    {{- include "chart.selectorLabels" . | nindent 4 }}
`
}

func (d *Deployer) helmHPAYaml() string {
	return `{{- if .Values.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "chart.fullname" . }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetCPUUtilizationPercentage }}
{{- end }}
`
}

func (d *Deployer) helmNetworkPolicyYaml() string {
	return `{{- if .Values.networkPolicy.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "chart.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from: []
      ports:
        - protocol: TCP
          port: http
  egress:
    - to: []
      ports:
        - protocol: TCP
          port: 443
        - protocol: TCP
          port: 80
{{- end }}
`
}

func (d *Deployer) helmServiceMonitorYaml() string {
	return `{{- if .Values.serviceMonitor.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "chart.selectorLabels" . | nindent 6 }}
  endpoints:
    - port: http
      path: /metrics
      interval: {{ .Values.serviceMonitor.interval }}
{{- end }}
`
}

func (d *Deployer) helmSecretYaml() string {
	return `apiVersion: v1
kind: Secret
metadata:
  name: {{ include "chart.fullname" . }}-secret
  labels:
    {{- include "chart.labels" . | nindent 4 }}
type: Opaque
data:
  api-key: {{ .Values.env | values | first | b64enc | quote }}
`
}

func (d *Deployer) helmHelpersYaml() string {
	return `{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chart.labels" -}}
helm.sh/chart: {{ include "chart.name" . }}
{{ include "chart.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
`
}

func (d *Deployer) deployKubernetes() (*DeployResult, error) {
	data := d.buildHelmData()

	chartDir := fmt.Sprintf("kubernetes/%s", data.ChartName)
	templatesDir := chartDir + "/templates"

	// Render Chart.yaml and values.yaml through Go templates (they contain agent-specific values).
	chartYaml, err := renderTemplate("Chart.yaml", helmChartYamlTmpl, data)
	if err != nil {
		return nil, fmt.Errorf("render Chart.yaml: %w", err)
	}

	valuesYaml, err := renderTemplate("values.yaml", helmValuesTmpl, data)
	if err != nil {
		return nil, fmt.Errorf("render values.yaml: %w", err)
	}

	// Helm template files are written as literal strings — they use Helm's own {{ }} syntax.
	files := []struct {
		path    string
		content string
	}{
		{chartDir + "/Chart.yaml", chartYaml},
		{chartDir + "/values.yaml", valuesYaml},
		{templatesDir + "/_helpers.tpl", d.helmHelpersYaml()},
		{templatesDir + "/deployment.yaml", d.helmDeploymentYaml()},
		{templatesDir + "/service.yaml", d.helmServiceYaml()},
		{templatesDir + "/hpa.yaml", d.helmHPAYaml()},
		{templatesDir + "/networkpolicy.yaml", d.helmNetworkPolicyYaml()},
		{templatesDir + "/servicemonitor.yaml", d.helmServiceMonitorYaml()},
		{templatesDir + "/secret.yaml", d.helmSecretYaml()},
	}

	for _, f := range files {
		if err := d.writeFile(f.path, f.content); err != nil {
			return nil, err
		}
	}

	deployedFiles := make([]DeployedFile, len(files))
	descriptions := []string{
		"Helm chart metadata",
		"Default values (replicas, resources, scaling)",
		"Template helper functions",
		"Deployment with security contexts and sidecars",
		"ClusterIP service",
		"HorizontalPodAutoscaler",
		"NetworkPolicy (ingress/egress)",
		"ServiceMonitor for Prometheus",
		"Secret template for API key",
	}
	for i, f := range files {
		deployedFiles[i] = DeployedFile{Path: f.path, Description: descriptions[i]}
	}

	return &DeployResult{
		Target: TargetKubernetes,
		Files:  deployedFiles,
		NextSteps: []string{
			fmt.Sprintf("helm install %s deploy/kubernetes/%s", data.ChartName, data.ChartName),
			fmt.Sprintf("helm upgrade %s deploy/kubernetes/%s --set env.%s=<your-key>", data.ChartName, data.ChartName, data.APIKeyEnv),
			"kubectl get pods -l app.kubernetes.io/name=" + data.ChartName,
		},
	}, nil
}
