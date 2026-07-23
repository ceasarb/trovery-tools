package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

func testConfig() *config.AgentConfig {
	return &config.AgentConfig{
		Name: "test-agent",
		Model: config.ModelConfig{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			Model:       "claude-haiku-4-5-20251001",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		System: "You are a test assistant.",
		Servers: []config.ServerRef{
			{
				Name:    "weather",
				Path:    "/tmp/weather",
				Command: "uv run weather",
			},
			{
				Name: "search-api",
				URL:  "https://search.example.com/mcp",
			},
		},
		Settings: config.AgentSettings{
			MaxToolCalls:     25,
			TimeoutSecs:      120,
			Namespacing:      "auto",
			BudgetPerRequest: 0.50,
			BudgetMonthly:    100.0,
		},
	}
}

func testConfigOpenAI() *config.AgentConfig {
	cfg := testConfig()
	cfg.Model.Provider = "openai"
	cfg.Model.APIKeyEnv = "OPENAI_API_KEY"
	cfg.Model.Model = "gpt-4o"
	return cfg
}

func testConfigNoServers() *config.AgentConfig {
	cfg := testConfig()
	cfg.Servers = nil
	return cfg
}

// --- Target Validation ---

func TestIsValidTarget(t *testing.T) {
	valid := []string{"docker", "kubernetes", "cloud-run", "container-apps", "github-actions", "all"}
	for _, tgt := range valid {
		if !IsValidTarget(tgt) {
			t.Errorf("IsValidTarget(%q) = false, want true", tgt)
		}
	}

	invalid := []string{"heroku", "lambda", "", "Docker"}
	for _, tgt := range invalid {
		if IsValidTarget(tgt) {
			t.Errorf("IsValidTarget(%q) = true, want false", tgt)
		}
	}
}

func TestDeployUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, Target("unknown"), dir)
	_, err := d.Deploy()
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestDeployCreatesOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "output")
	cfg := testConfig()

	d := New(cfg, TargetDocker, dir)
	_, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

// --- Docker Target ---

func TestDeployDocker(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetDocker, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}

	result := results[0]
	if result.Target != TargetDocker {
		t.Errorf("Target = %q, want %q", result.Target, TargetDocker)
	}
	if len(result.Files) != 4 {
		t.Errorf("Files count = %d, want 4", len(result.Files))
	}

	// Verify Dockerfile is hardened
	dockerfile := readFile(t, dir, "docker/Dockerfile")
	assertContains(t, dockerfile, "distroless")
	assertContains(t, dockerfile, "nonroot")
	assertContains(t, dockerfile, "USER nonroot:nonroot")
	assertContains(t, dockerfile, "test-agent")

	// Verify compose has resource limits
	compose := readFile(t, dir, "docker/docker-compose.yml")
	assertContains(t, compose, "weather")
	assertContains(t, compose, "ANTHROPIC_API_KEY")
	assertContains(t, compose, "memory: 256M")
	assertContains(t, compose, "healthcheck")
	assertContains(t, compose, "read_only: true")

	// Verify .env.example
	envExample := readFile(t, dir, "docker/.env.example")
	assertContains(t, envExample, "ANTHROPIC_API_KEY")

	// Verify next steps
	if len(result.NextSteps) == 0 {
		t.Error("expected next steps")
	}
}

func TestDeployDocker_NoServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigNoServers()

	d := New(cfg, TargetDocker, dir)
	_, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	compose := readFile(t, dir, "docker/docker-compose.yml")
	assertNotContains(t, compose, "depends_on")
}

func TestDeployDocker_OpenAI(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigOpenAI()

	d := New(cfg, TargetDocker, dir)
	_, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	envExample := readFile(t, dir, "docker/.env.example")
	assertContains(t, envExample, "OPENAI_API_KEY")
	assertNotContains(t, envExample, "ANTHROPIC")
}

// --- Kubernetes Target ---

func TestDeployKubernetes(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetKubernetes, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	result := results[0]
	if result.Target != TargetKubernetes {
		t.Errorf("Target = %q, want %q", result.Target, TargetKubernetes)
	}
	if len(result.Files) != 9 {
		t.Errorf("Files count = %d, want 9", len(result.Files))
	}

	// Verify Chart.yaml
	chartYaml := readFile(t, dir, "kubernetes/test-agent/Chart.yaml")
	assertContains(t, chartYaml, "name: test-agent")
	assertContains(t, chartYaml, "apiVersion: v2")

	// Verify values.yaml
	valuesYaml := readFile(t, dir, "kubernetes/test-agent/values.yaml")
	assertContains(t, valuesYaml, "replicaCount: 1")
	assertContains(t, valuesYaml, "ANTHROPIC_API_KEY")
	assertContains(t, valuesYaml, "memory: 256Mi")
	assertContains(t, valuesYaml, "networkPolicy")
	assertContains(t, valuesYaml, "serviceMonitor")
	// Bundled server sidecar
	assertContains(t, valuesYaml, "weather")

	// Verify deployment template has security contexts
	deployment := readFile(t, dir, "kubernetes/test-agent/templates/deployment.yaml")
	assertContains(t, deployment, "runAsNonRoot: true")
	assertContains(t, deployment, "readOnlyRootFilesystem: true")
	assertContains(t, deployment, "allowPrivilegeEscalation: false")
	assertContains(t, deployment, "/health")
	assertContains(t, deployment, "/ready")

	// Verify service
	service := readFile(t, dir, "kubernetes/test-agent/templates/service.yaml")
	assertContains(t, service, "kind: Service")
	assertContains(t, service, ".Values.service.type")

	// Verify HPA
	hpa := readFile(t, dir, "kubernetes/test-agent/templates/hpa.yaml")
	assertContains(t, hpa, "HorizontalPodAutoscaler")

	// Verify NetworkPolicy
	np := readFile(t, dir, "kubernetes/test-agent/templates/networkpolicy.yaml")
	assertContains(t, np, "NetworkPolicy")
	assertContains(t, np, "Ingress")
	assertContains(t, np, "Egress")

	// Verify ServiceMonitor
	sm := readFile(t, dir, "kubernetes/test-agent/templates/servicemonitor.yaml")
	assertContains(t, sm, "ServiceMonitor")
	assertContains(t, sm, "/metrics")

	// Verify helpers
	helpers := readFile(t, dir, "kubernetes/test-agent/templates/_helpers.tpl")
	assertContains(t, helpers, "chart.fullname")
	assertContains(t, helpers, "chart.labels")
}

func TestDeployKubernetes_NoServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigNoServers()

	d := New(cfg, TargetKubernetes, dir)
	_, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	valuesYaml := readFile(t, dir, "kubernetes/test-agent/values.yaml")
	assertNotContains(t, valuesYaml, "sidecars:")
}

// --- Cloud Run Target ---

func TestDeployCloudRun(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetCloudRun, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	result := results[0]
	if result.Target != TargetCloudRun {
		t.Errorf("Target = %q, want %q", result.Target, TargetCloudRun)
	}
	if len(result.Files) != 4 {
		t.Errorf("Files count = %d, want 4", len(result.Files))
	}

	// Verify main.tf
	mainTf := readFile(t, dir, "cloud-run/main.tf")
	assertContains(t, mainTf, "google_cloud_run_v2_service")
	assertContains(t, mainTf, "google_secret_manager_secret")
	assertContains(t, mainTf, "google_service_account")
	assertContains(t, mainTf, "ANTHROPIC_API_KEY")
	assertContains(t, mainTf, "test-agent")
	assertContains(t, mainTf, "/health")
	assertContains(t, mainTf, "hashicorp/google")

	// Verify variables.tf
	varsTf := readFile(t, dir, "cloud-run/variables.tf")
	assertContains(t, varsTf, "project_id")
	assertContains(t, varsTf, "region")
	assertContains(t, varsTf, "image")
	assertContains(t, varsTf, "api_key")
	assertContains(t, varsTf, "sensitive")

	// Verify outputs.tf
	outputsTf := readFile(t, dir, "cloud-run/outputs.tf")
	assertContains(t, outputsTf, "service_url")

	// Verify tfvars example
	tfvars := readFile(t, dir, "cloud-run/terraform.tfvars.example")
	assertContains(t, tfvars, "your-gcp-project")
	assertContains(t, tfvars, "test-agent")
}

// --- Container Apps Target ---

func TestDeployContainerApps(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetContainerApps, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	result := results[0]
	if result.Target != TargetContainerApps {
		t.Errorf("Target = %q, want %q", result.Target, TargetContainerApps)
	}
	if len(result.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(result.Files))
	}

	// Verify main.bicep
	bicep := readFile(t, dir, "container-apps/main.bicep")
	assertContains(t, bicep, "Microsoft.App/containerApps")
	assertContains(t, bicep, "Microsoft.KeyVault/vaults")
	assertContains(t, bicep, "Microsoft.ManagedIdentity")
	assertContains(t, bicep, "ANTHROPIC_API_KEY")
	assertContains(t, bicep, "/health")
	assertContains(t, bicep, "/ready")
	assertContains(t, bicep, "test-agent")
	assertContains(t, bicep, "@secure()")

	// Verify parameters.json
	params := readFile(t, dir, "container-apps/parameters.json")
	assertContains(t, params, "test-agent")

	// Validate JSON structure
	var parsed map[string]any
	if err := json.Unmarshal([]byte(params), &parsed); err != nil {
		t.Fatalf("parameters.json is not valid JSON: %v", err)
	}
}

// --- GitHub Actions Target ---

func TestDeployGitHubActions(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetGitHubActions, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	result := results[0]
	if result.Target != TargetGitHubActions {
		t.Errorf("Target = %q, want %q", result.Target, TargetGitHubActions)
	}
	if len(result.Files) != 4 {
		t.Errorf("Files count = %d, want 4", len(result.Files))
	}

	// Verify validate.yml
	validate := readFile(t, dir, ".github/workflows/validate.yml")
	assertContains(t, validate, "Validate")
	assertContains(t, validate, "pull_request")
	assertContains(t, validate, "demi forge server validate")
	assertContains(t, validate, "demi forge server test")

	// Verify eval.yml
	eval := readFile(t, dir, ".github/workflows/eval.yml")
	assertContains(t, eval, "Eval")
	assertContains(t, eval, "push")
	assertContains(t, eval, "demi forge agent eval")
	assertContains(t, eval, "ANTHROPIC_API_KEY")

	// Verify deploy.yml
	deployWf := readFile(t, dir, ".github/workflows/deploy.yml")
	assertContains(t, deployWf, "Deploy")
	assertContains(t, deployWf, "tags:")
	assertContains(t, deployWf, "docker/build-push-action")
	assertContains(t, deployWf, "ghcr.io")

	// Verify benchmark.yml
	benchWf := readFile(t, dir, ".github/workflows/benchmark.yml")
	assertContains(t, benchWf, "Benchmark")
	assertContains(t, benchWf, "bench")
	assertContains(t, benchWf, "benchstat")
}

// --- All Target ---

func TestDeployAll(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetAll, dir)
	results, err := d.Deploy()
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	// 5 targets + 1 observability
	if len(results) != 6 {
		t.Errorf("results count = %d, want 6", len(results))
	}

	// Verify all targets are represented
	targets := make(map[Target]bool)
	for _, r := range results {
		targets[r.Target] = true
	}
	for _, expected := range ValidTargets() {
		if !targets[expected] {
			t.Errorf("missing target: %s", expected)
		}
	}

	// Verify observability target is included
	if !targets["observability"] {
		t.Error("missing observability target in 'all' deploy")
	}
}

// --- Observability Artifacts ---

func TestGenerateObservabilityArtifacts(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	d := New(cfg, TargetDocker, dir) // target doesn't matter for this test
	files, err := d.GenerateObservabilityArtifacts()
	if err != nil {
		t.Fatalf("GenerateObservabilityArtifacts: %v", err)
	}

	if len(files) != 4 {
		t.Errorf("files count = %d, want 4", len(files))
	}

	// Verify Grafana dashboard is valid JSON
	dashboard := readFile(t, dir, "observability/grafana-dashboard.json")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(dashboard), &parsed); err != nil {
		t.Fatalf("grafana dashboard is not valid JSON: %v", err)
	}
	assertContains(t, dashboard, "test-agent")
	assertContains(t, dashboard, "demi_requests_total")
	assertContains(t, dashboard, "demi_cost_usd_total")

	// Verify Prometheus alert rules
	alerts := readFile(t, dir, "observability/prometheus-alerts.yml")
	assertContains(t, alerts, "HighErrorRate")
	assertContains(t, alerts, "HighLatency")
	assertContains(t, alerts, "BudgetLow")
	assertContains(t, alerts, "test-agent")

	// Verify GCP alert policy
	gcpAlerts := readFile(t, dir, "observability/gcp-alerts.yaml")
	assertContains(t, gcpAlerts, "test-agent")
	assertContains(t, gcpAlerts, "cloud_run")

	// Verify Azure alert rules
	azureAlerts := readFile(t, dir, "observability/azure-alerts.bicep")
	assertContains(t, azureAlerts, "test-agent")
	assertContains(t, azureAlerts, "Microsoft.Insights/metricAlerts")
}

// --- Server Classification ---

func TestClassifyServers(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, TargetDocker, t.TempDir())

	bundled, external := d.classifyServers()

	if len(bundled) != 1 {
		t.Errorf("bundled count = %d, want 1", len(bundled))
	}
	if len(external) != 1 {
		t.Errorf("external count = %d, want 1", len(external))
	}
	if bundled[0].Name != "weather" {
		t.Errorf("bundled[0].Name = %q, want %q", bundled[0].Name, "weather")
	}
	if external[0].Name != "search-api" {
		t.Errorf("external[0].Name = %q, want %q", external[0].Name, "search-api")
	}
}

func TestAPIKeyEnv_Default(t *testing.T) {
	cfg := testConfig()
	cfg.Model.APIKeyEnv = ""
	cfg.Model.Provider = "anthropic"
	d := New(cfg, TargetDocker, t.TempDir())

	if got := d.apiKeyEnv(); got != "ANTHROPIC_API_KEY" {
		t.Errorf("apiKeyEnv() = %q, want ANTHROPIC_API_KEY", got)
	}

	cfg.Model.Provider = "openai"
	d2 := New(cfg, TargetDocker, t.TempDir())
	if got := d2.apiKeyEnv(); got != "OPENAI_API_KEY" {
		t.Errorf("apiKeyEnv() = %q, want OPENAI_API_KEY", got)
	}

	cfg.Model.Provider = "custom"
	d3 := New(cfg, TargetDocker, t.TempDir())
	if got := d3.apiKeyEnv(); got != "CUSTOM_API_KEY" {
		t.Errorf("apiKeyEnv() = %q, want CUSTOM_API_KEY", got)
	}
}

// --- Helpers ---

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func assertContains(t *testing.T, content, substr string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("expected content to contain %q, but it didn't.\nContent (first 500 chars):\n%s", substr, truncate(content, 500))
	}
}

func assertNotContains(t *testing.T, content, substr string) {
	t.Helper()
	if strings.Contains(content, substr) {
		t.Errorf("expected content to NOT contain %q, but it did", substr)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
