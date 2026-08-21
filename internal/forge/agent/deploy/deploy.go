package deploy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

// Target identifies the deployment target.
type Target string

const (
	TargetDocker        Target = "docker"
	TargetKubernetes    Target = "kubernetes"
	TargetCloudRun      Target = "cloud-run"
	TargetContainerApps Target = "container-apps"
	TargetGitHubActions Target = "github-actions"
	TargetAll           Target = "all"
)

// ValidTargets returns all supported deployment targets (excluding "all").
func ValidTargets() []Target {
	return []Target{
		TargetDocker,
		TargetKubernetes,
		TargetCloudRun,
		TargetContainerApps,
		TargetGitHubActions,
	}
}

// IsValidTarget checks whether t is a recognized target.
func IsValidTarget(t string) bool {
	if t == string(TargetAll) {
		return true
	}
	for _, v := range ValidTargets() {
		if string(v) == t {
			return true
		}
	}
	return false
}

// DeployResult describes the files produced by a deployment target.
type DeployResult struct {
	Target    Target
	Files     []DeployedFile
	NextSteps []string
}

// DeployedFile is a single file written during deploy generation.
type DeployedFile struct {
	Path        string
	Description string
}

// Deployer generates production deployment artifacts from an agent config.
type Deployer struct {
	config *config.AgentConfig
	target Target
	outDir string
}

// New creates a Deployer for the given config, target, and output directory.
func New(cfg *config.AgentConfig, target Target, outDir string) *Deployer {
	return &Deployer{
		config: cfg,
		target: target,
		outDir: outDir,
	}
}

// Deploy generates deployment artifacts and writes them to disk.
// When target is "all", it runs every target and merges results.
func (d *Deployer) Deploy() ([]*DeployResult, error) {
	if err := os.MkdirAll(d.outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	if d.target == TargetAll {
		var results []*DeployResult
		for _, t := range ValidTargets() {
			sub := &Deployer{config: d.config, target: t, outDir: d.outDir}
			r, err := sub.deploySingle()
			if err != nil {
				return nil, fmt.Errorf("target %s: %w", t, err)
			}
			results = append(results, r)
		}

		// Generate observability artifacts (Grafana dashboard, alert rules)
		obsFiles, err := d.GenerateObservabilityArtifacts()
		if err != nil {
			return nil, fmt.Errorf("observability artifacts: %w", err)
		}
		results = append(results, &DeployResult{
			Target: "observability",
			Files:  obsFiles,
			NextSteps: []string{
				"Import observability/grafana-dashboard.json into Grafana",
				"Add observability/prometheus-alerts.yml to Prometheus rule_files",
			},
		})

		return results, nil
	}

	r, err := d.deploySingle()
	if err != nil {
		return nil, err
	}
	return []*DeployResult{r}, nil
}

func (d *Deployer) deploySingle() (*DeployResult, error) {
	switch d.target {
	case TargetDocker:
		return d.deployDocker()
	case TargetKubernetes:
		return d.deployKubernetes()
	case TargetCloudRun:
		return d.deployCloudRun()
	case TargetContainerApps:
		return d.deployContainerApps()
	case TargetGitHubActions:
		return d.deployGitHubActions()
	default:
		return nil, fmt.Errorf("unsupported target: %s", d.target)
	}
}

// classifyServers separates servers into bundled (local command) and external (URL).
func (d *Deployer) classifyServers() (bundled, external []config.ServerRef) {
	for _, s := range d.config.Servers {
		if s.URL != "" {
			external = append(external, s)
		} else {
			bundled = append(bundled, s)
		}
	}
	return
}

// apiKeyEnv returns the environment variable name for the provider's API key.
func (d *Deployer) apiKeyEnv() string {
	if d.config.Model.APIKeyEnv != "" {
		return d.config.Model.APIKeyEnv
	}
	switch d.config.Model.Provider {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	default:
		return strings.ToUpper(d.config.Model.Provider) + "_API_KEY"
	}
}

// renderTemplate renders a Go text/template string with the given data.
func renderTemplate(name, tmpl string, data any) (string, error) {
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
	}
	t, err := template.New(name).Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// writeFile writes content to a file inside the deployer's output directory.
func (d *Deployer) writeFile(relPath, content string) error {
	path := filepath.Join(d.outDir, relPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
