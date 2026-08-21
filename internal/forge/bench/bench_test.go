package bench

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/deploy"
	agentruntime "github.com/ceasarb/trovery-tools/pkg/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/internal/forge/server/scaffold"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/storage"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/templates"
)

// --- CLI Startup Benchmark ---
// Target: <100ms

func BenchmarkCLIStartup(b *testing.B) {
	// Build the binary first
	binPath := filepath.Join(b.TempDir(), "trove-forge")
	build := exec.Command("go", "build", "-o", binPath, "github.com/ceasarb/trovery-tools/cmd/trove-forge")
	if out, err := build.CombinedOutput(); err != nil {
		b.Fatalf("build: %v\n%s", err, out)
	}

	b.ResetTimer()
	for range b.N {
		cmd := exec.Command(binPath, "--version")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			b.Fatalf("run: %v", err)
		}
	}
}

// --- Template Rendering Benchmarks ---
// Target: <500ms per template

func BenchmarkTemplatePythonStdio(b *testing.B) {
	benchmarkScaffold(b, scaffold.Python, scaffold.Stdio)
}

func BenchmarkTemplatePythonHTTP(b *testing.B) {
	benchmarkScaffold(b, scaffold.Python, scaffold.HTTP)
}

func BenchmarkTemplateTypeScriptStdio(b *testing.B) {
	benchmarkScaffold(b, scaffold.TypeScript, scaffold.Stdio)
}

func benchmarkScaffold(b *testing.B, lang scaffold.Language, transport scaffold.Transport) {
	b.Helper()
	for range b.N {
		dir := b.TempDir()
		_, err := scaffold.Run(scaffold.Options{
			Name:      "bench-server",
			Language:  lang,
			Transport: transport,
			OutputDir: dir,
		})
		if err != nil {
			b.Fatalf("scaffold: %v", err)
		}
	}
}

// --- Template Render (raw) ---

func BenchmarkTemplateRender(b *testing.B) {
	tmpl := `name: {{ .ServiceName }}
package: {{ .PythonPackage }}
prefix: {{ .ToolPrefix }}
transport: {{ .Transport }}
description: {{ .Description }}`

	ctx := templates.Context{
		ServiceName:   "bench-server",
		PythonPackage: "bench_server",
		ToolPrefix:    "bench_server",
		Transport:     "stdio",
		Description:   "A benchmark test server",
	}

	b.ResetTimer()
	for range b.N {
		_, err := templates.Render(tmpl, ctx)
		if err != nil {
			b.Fatalf("render: %v", err)
		}
	}
}

// --- Deploy Artifact Generation ---

func BenchmarkDeployDocker(b *testing.B) {
	benchmarkDeploy(b, deploy.TargetDocker)
}

func BenchmarkDeployKubernetes(b *testing.B) {
	benchmarkDeploy(b, deploy.TargetKubernetes)
}

func BenchmarkDeployCloudRun(b *testing.B) {
	benchmarkDeploy(b, deploy.TargetCloudRun)
}

func BenchmarkDeployAll(b *testing.B) {
	benchmarkDeploy(b, deploy.TargetAll)
}

func benchmarkDeploy(b *testing.B, target deploy.Target) {
	b.Helper()
	cfg := &config.AgentConfig{
		Name: "bench-agent",
		Model: config.ModelConfig{
			Provider: "anthropic",
			Model:    "claude-haiku-4-5-20251001",
		},
		Servers: []config.ServerRef{
			{Name: "server-a", Command: "uv run server-a"},
			{Name: "server-b", URL: "https://api.example.com/mcp"},
		},
	}

	b.ResetTimer()
	for range b.N {
		dir := b.TempDir()
		d := deploy.New(cfg, target, dir)
		if _, err := d.Deploy(); err != nil {
			b.Fatalf("deploy: %v", err)
		}
	}
}

// --- Agent Runtime Session Creation ---

func BenchmarkSessionCreation(b *testing.B) {
	cfg := &config.AgentConfig{
		Name: "bench-agent",
		Model: config.ModelConfig{
			Provider:  "anthropic",
			Model:     "claude-haiku-4-5-20251001",
			MaxTokens: 4096,
		},
		Settings: config.AgentSettings{
			MaxToolCalls: 25,
			TimeoutSecs:  60,
		},
	}

	b.ResetTimer()
	for range b.N {
		sess := agentruntime.NewSession(cfg, nil, nil)
		sess.Output = agentruntime.SilentOutput()
		_ = sess
	}
}

// --- SQLite Storage Operations ---

func BenchmarkSQLiteEvalStore(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench_evals.db")
	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := range b.N {
		run := &storage.EvalRun{
			ID:         fmt.Sprintf("run-%d", i),
			Source:     "server",
			TargetName: "bench-server",
			SuiteName:  "bench-suite",
			StartedAt:  time.Now(),
			Status:     "running",
		}
		if err := store.CreateRun(run); err != nil {
			b.Fatalf("create run: %v", err)
		}
		dur := int64(100)
		_ = store.CreateResult(&storage.EvalResult{
			ID: fmt.Sprintf("res-%d-1", i), RunID: run.ID,
			ScenarioName: "scenario-1", Status: "passed", DurationMs: &dur,
		})
	}
}

func BenchmarkSQLiteSessionStore(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench_sessions.db")
	store, err := storage.NewSessionStore(dbPath)
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := range b.N {
		sess := &storage.Session{
			ID:        fmt.Sprintf("sess-%d", i),
			AgentName: "bench-agent",
			Provider:  "anthropic",
			Model:     "claude-haiku-4-5-20251001",
			StartedAt: time.Now(),
		}
		if err := store.CreateSession(sess); err != nil {
			b.Fatalf("create session: %v", err)
		}
		_ = store.CreateTurn(&storage.SessionTurn{
			ID: fmt.Sprintf("turn-%d", i), SessionID: sess.ID,
			TurnNumber: 1, Role: "user", Content: "hello",
		})
	}
}

// --- Dashboard HTTP Handler ---

func BenchmarkDashboardHealthEndpoint(b *testing.B) {
	// Create a minimal dashboard-like handler
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}
}

// --- Binary Size Check (not a benchmark, but a test) ---

func TestBinarySizeUnder50MB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary size check in short mode")
	}

	binPath := filepath.Join(t.TempDir(), "trove-forge")
	build := exec.Command("go", "build", "-ldflags=-s -w", "-o", binPath, "github.com/ceasarb/trovery-tools/cmd/trove-forge")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	t.Logf("Binary size: %.1f MB (%s/%s)", sizeMB, runtime.GOOS, runtime.GOARCH)

	if sizeMB > 50 {
		t.Errorf("binary size %.1f MB exceeds 50MB target", sizeMB)
	}
}

// --- CLI Startup Time Check ---

func TestCLIStartupUnder100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping startup time check in short mode")
	}

	binPath := filepath.Join(t.TempDir(), "trove-forge")
	build := exec.Command("go", "build", "-o", binPath, "github.com/ceasarb/trovery-tools/cmd/trove-forge")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// Warm up
	exec.Command(binPath, "--version").Run()

	// Measure 5 runs
	var total time.Duration
	runs := 5
	for range runs {
		start := time.Now()
		cmd := exec.Command(binPath, "--version")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		total += time.Since(start)
	}

	avg := total / time.Duration(runs)
	t.Logf("Average CLI startup: %v (over %d runs)", avg, runs)

	if avg > 100*time.Millisecond {
		t.Errorf("average CLI startup %v exceeds 100ms target", avg)
	}
}
