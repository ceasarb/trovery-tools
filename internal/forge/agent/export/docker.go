package export

import (
	"fmt"
	"strings"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

const dockerfileTmpl = `FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
`

const dockerIgnoreTmpl = `__pycache__
*.pyc
.env
.venv
venv
.git
`

const envExampleTmpl = `# {{ .AgentName }} — Environment Variables
{{ if eq .Provider "anthropic" }}ANTHROPIC_API_KEY=sk-ant-...{{ else }}OPENAI_API_KEY=sk-...{{ end }}
`

type dockerComposeData struct {
	AgentName       string
	Provider        string
	APIKeyEnv       string
	BundledServers  []config.ServerRef
	ExternalServers []config.ServerRef
	HasBundled      bool
}

const dockerComposeTmpl = `services:
  agent:
    build: .
    ports:
      - "8000:8000"
    env_file:
      - .env
    environment:
      - {{ .APIKeyEnv }}=${{"{"}}{{ .APIKeyEnv }}{{"}"}}
{{ if .HasBundled }}    depends_on:
{{ range .BundledServers }}      {{ .Name }}:
        condition: service_started
{{ end }}
{{ range .BundledServers }}  {{ .Name }}:
    image: python:3.13-slim
    command: {{ .Command }}
    working_dir: /app
{{ end }}{{ end }}`

func (e *Exporter) exportDocker() (*ExportResult, error) {
	bundled, external := e.classifyServers()

	// First generate the FastAPI app as the base
	fastapiData, err := e.buildPythonData()
	if err != nil {
		return nil, fmt.Errorf("build fastapi data: %w", err)
	}

	app, err := renderTemplate("app.py", fastapiAppTmpl, fastapiData)
	if err != nil {
		return nil, fmt.Errorf("render app.py: %w", err)
	}

	reqs, err := renderTemplate("requirements.txt", fastapiRequirementsTmpl, fastapiData)
	if err != nil {
		return nil, fmt.Errorf("render requirements.txt: %w", err)
	}

	// Determine API key env var name
	apiKeyEnv := e.config.Model.APIKeyEnv
	if apiKeyEnv == "" {
		switch e.config.Model.Provider {
		case "anthropic":
			apiKeyEnv = "ANTHROPIC_API_KEY"
		case "openai":
			apiKeyEnv = "OPENAI_API_KEY"
		default:
			apiKeyEnv = strings.ToUpper(e.config.Model.Provider) + "_API_KEY"
		}
	}

	composeData := dockerComposeData{
		AgentName:       e.config.Name,
		Provider:        e.config.Model.Provider,
		APIKeyEnv:       apiKeyEnv,
		BundledServers:  bundled,
		ExternalServers: external,
		HasBundled:      len(bundled) > 0,
	}

	compose, err := renderTemplate("docker-compose.yml", dockerComposeTmpl, composeData)
	if err != nil {
		return nil, fmt.Errorf("render docker-compose.yml: %w", err)
	}

	envExample, err := renderTemplate(".env.example", envExampleTmpl, fastapiData)
	if err != nil {
		return nil, fmt.Errorf("render .env.example: %w", err)
	}

	files := []struct {
		name    string
		content string
	}{
		{"app.py", app},
		{"requirements.txt", reqs},
		{"Dockerfile", dockerfileTmpl},
		{"docker-compose.yml", compose},
		{".dockerignore", dockerIgnoreTmpl},
		{".env.example", envExample},
	}

	for _, f := range files {
		if err := e.writeFile(f.name, f.content); err != nil {
			return nil, err
		}
	}

	return &ExportResult{
		Format: FormatDocker,
		Files: []ExportedFile{
			{Path: "app.py", Description: "FastAPI application"},
			{Path: "requirements.txt", Description: "Python dependencies"},
			{Path: "Dockerfile", Description: "Container image definition"},
			{Path: "docker-compose.yml", Description: "Compose stack with agent" + bundledDesc(bundled)},
			{Path: ".dockerignore", Description: "Docker ignore rules"},
			{Path: ".env.example", Description: "Environment variable template"},
		},
	}, nil
}

func bundledDesc(bundled []config.ServerRef) string {
	if len(bundled) == 0 {
		return ""
	}
	names := make([]string, len(bundled))
	for i, s := range bundled {
		names[i] = s.Name
	}
	return fmt.Sprintf(" + %s", strings.Join(names, ", "))
}
