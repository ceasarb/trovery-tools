package deploy

import (
	"fmt"
	"strings"

	"github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

type dockerData struct {
	AgentName       string
	Provider        string
	APIKeyEnv       string
	Port            int
	BundledServers  []config.ServerRef
	ExternalServers []config.ServerRef
	HasBundled      bool
	BudgetPerReq    float64
	BudgetMonthly   float64
}

func (d *Deployer) buildDockerData() dockerData {
	bundled, external := d.classifyServers()
	port := 8080
	if d.config.Settings.TimeoutSecs > 0 {
		port = 8080
	}
	return dockerData{
		AgentName:       d.config.Name,
		Provider:        d.config.Model.Provider,
		APIKeyEnv:       d.apiKeyEnv(),
		Port:            port,
		BundledServers:  bundled,
		ExternalServers: external,
		HasBundled:      len(bundled) > 0,
		BudgetPerReq:    d.config.Settings.BudgetPerRequest,
		BudgetMonthly:   d.config.Settings.BudgetMonthly,
	}
}

// Hardened multi-stage Dockerfile for the Go agent binary.
const hardenedDockerfileTmpl = `# --- Builder stage ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /demi-forge ./cmd/demi-forge

# --- Runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /demi-forge /usr/local/bin/demi-forge
COPY agents/{{ .AgentName }}/ /app/agents/{{ .AgentName }}/

WORKDIR /app

EXPOSE {{ .Port }}

USER nonroot:nonroot

ENTRYPOINT ["demi-forge", "agent", "serve", "{{ .AgentName }}", "--host", "0.0.0.0", "--port", "{{ .Port }}"]
`

const hardenedDockerIgnoreTmpl = `.git
.gitignore
*.md
deploy/
export/
.env
.venv
venv
__pycache__
*.pyc
node_modules
.DS_Store
`

const hardenedEnvExampleTmpl = `# {{ .AgentName }} — Environment Variables
#
# Provider API key (required)
{{ .APIKeyEnv }}=
{{ if gt .BudgetMonthly 0.0 }}
# Cost guardrails
# AJNT_BUDGET_MONTHLY={{ printf "%.2f" .BudgetMonthly }}
{{ end }}`

const hardenedComposeTmpl = `services:
  agent:
    build:
      context: .
      dockerfile: deploy/Dockerfile
    ports:
      - "{{ .Port }}:{{ .Port }}"
    env_file:
      - .env
    environment:
      - {{ .APIKeyEnv }}=${"{"}}{{ .APIKeyEnv }}{{"}"}}
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "1.0"
    healthcheck:
      test: ["CMD", "/usr/local/bin/demi-forge", "--version"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: unless-stopped
{{ if .HasBundled }}    depends_on:
{{ range .BundledServers }}      {{ .Name }}:
        condition: service_started
{{ end }}
{{ range .BundledServers }}  {{ .Name }}:
    build:
      context: servers/{{ .Name }}
    read_only: true
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.5"
    restart: unless-stopped
{{ end }}{{ end }}`

func (d *Deployer) deployDocker() (*DeployResult, error) {
	data := d.buildDockerData()

	dockerfile, err := renderTemplate("Dockerfile", hardenedDockerfileTmpl, data)
	if err != nil {
		return nil, fmt.Errorf("render Dockerfile: %w", err)
	}

	compose, err := renderTemplate("docker-compose.yml", hardenedComposeTmpl, data)
	if err != nil {
		return nil, fmt.Errorf("render docker-compose.yml: %w", err)
	}

	envExample, err := renderTemplate(".env.example", hardenedEnvExampleTmpl, data)
	if err != nil {
		return nil, fmt.Errorf("render .env.example: %w", err)
	}

	files := []struct {
		name    string
		content string
	}{
		{"docker/Dockerfile", dockerfile},
		{"docker/docker-compose.yml", compose},
		{"docker/.dockerignore", hardenedDockerIgnoreTmpl},
		{"docker/.env.example", envExample},
	}

	for _, f := range files {
		if err := d.writeFile(f.name, f.content); err != nil {
			return nil, err
		}
	}

	nextSteps := []string{
		"cd deploy/docker",
		"cp .env.example .env  # fill in your API key",
		"docker compose up --build",
		fmt.Sprintf("curl http://localhost:%d/health", data.Port),
	}

	bundledDesc := ""
	if data.HasBundled {
		names := make([]string, len(data.BundledServers))
		for i, s := range data.BundledServers {
			names[i] = s.Name
		}
		bundledDesc = fmt.Sprintf(" + %s sidecars", strings.Join(names, ", "))
	}

	return &DeployResult{
		Target: TargetDocker,
		Files: []DeployedFile{
			{Path: "docker/Dockerfile", Description: "Multi-stage hardened image (distroless, non-root)"},
			{Path: "docker/docker-compose.yml", Description: "Compose stack" + bundledDesc},
			{Path: "docker/.dockerignore", Description: "Docker ignore rules"},
			{Path: "docker/.env.example", Description: "Environment variable template"},
		},
		NextSteps: nextSteps,
	}, nil
}
