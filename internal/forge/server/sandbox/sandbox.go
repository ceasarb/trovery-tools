package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/config"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/container"
)

// SandboxOpts configures a sandbox run.
type SandboxOpts struct {
	ServerDir string
	Config    *config.ServerConfig
	Policy    *SecurityPolicy
	Runtime   container.Runtime
	Port      string // optional port mapping, e.g. "8080:8080"
}

// Run builds and runs the server in a sandboxed container.
func Run(opts SandboxOpts) error {
	serverName := opts.Config.Server.Name
	imageTag := "demi-forge-sandbox-" + strings.ToLower(serverName)

	// Detect language
	lang, err := DetectLanguage(opts.ServerDir)
	if err != nil {
		return err
	}
	console.Dim(fmt.Sprintf("  Detected language: %s", lang))

	// Generate Dockerfile
	dockerfilePath, err := WriteDockerfile(opts.ServerDir, lang, opts.Config.Server.Command)
	if err != nil {
		return err
	}
	defer os.Remove(dockerfilePath)

	// Generate .dockerignore if not present
	diPath := filepath.Join(opts.ServerDir, ".dockerignore")
	if _, err := os.Stat(diPath); os.IsNotExist(err) {
		os.WriteFile(diPath, []byte(".venv\n.hdx\n.demi/forge\n__pycache__\n*.pyc\nnode_modules\n.git\n"), 0o644)
		defer os.Remove(diPath)
	}

	console.Dim("  Generated Dockerfile.demi")

	// Build image
	console.Info("Building sandbox image...")
	buildOpts := container.BuildOpts{
		ContextDir: opts.ServerDir,
		Dockerfile: dockerfilePath,
		Tag:        imageTag,
		Labels: map[string]string{
			"demi.sandbox": "true",
			"demi.server":  serverName,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := opts.Runtime.Build(ctx, buildOpts); err != nil {
		return fmt.Errorf("build sandbox image: %w", err)
	}
	console.Success("Image built: " + imageTag)

	// Configure run options from policy
	runOpts := container.RunOpts{
		Image:     imageTag,
		Name:      "demi-forge-sandbox-" + serverName,
		MemoryMB:  opts.Policy.MemoryMB,
		CPUs:      opts.Policy.CPUs,
		PidsLimit: opts.Policy.PidsLimit,
		ReadOnly:  opts.Policy.ReadOnly,
		Remove:    true,
		Detach:    true,
		Labels: map[string]string{
			"demi.sandbox": "true",
			"demi.server":  serverName,
			"demi.policy":  opts.Policy.Name,
		},
		Env: map[string]string{},
	}

	// Port mapping
	if opts.Port != "" {
		parts := strings.SplitN(opts.Port, ":", 2)
		if len(parts) == 2 {
			runOpts.Ports = map[string]string{parts[0]: parts[1]}
		} else {
			runOpts.Ports = map[string]string{opts.Port: opts.Port}
		}
	} else if opts.Config.Server.Port > 0 {
		port := fmt.Sprintf("%d", opts.Config.Server.Port)
		runOpts.Ports = map[string]string{port: port}
	}

	// Network policy — disable network entirely for strict
	if !opts.Policy.Network {
		runOpts.Env["AJNT_NETWORK_DISABLED"] = "1"
	}

	// Start container
	console.Info(fmt.Sprintf("Starting sandbox (%s policy)...", opts.Policy.Name))
	containerID, err := opts.Runtime.Run(ctx, runOpts)
	if err != nil {
		return fmt.Errorf("start sandbox container: %w", err)
	}
	console.Success(fmt.Sprintf("Sandbox running: %s", containerID[:12]))

	// Apply domain firewall if needed
	if opts.Policy.Network && len(opts.Policy.Domains) > 0 {
		script := GenerateFirewallScript(opts.Policy.Domains)
		if script != "" {
			console.Dim("  Applying domain firewall...")
			_, err := opts.Runtime.Exec(ctx, containerID, []string{"sh", "-c", script})
			if err != nil {
				console.Warning(fmt.Sprintf("Firewall script failed (may need --cap-add=NET_ADMIN): %v", err))
			}
		}
	}

	// Stream container logs in background
	go func() {
		logCmd := exec.CommandContext(ctx, opts.Runtime.Name(), "logs", "-f", containerID)
		logCmd.Stdout = os.Stdout
		logCmd.Stderr = os.Stderr
		logCmd.Run() //nolint:errcheck
	}()

	// Handle Ctrl+C for graceful cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	console.Dim("  Press Ctrl+C to stop")
	fmt.Println()

	<-sigCh
	fmt.Println()
	console.Info("Stopping sandbox...")

	cleanupCtx := context.Background()
	if err := opts.Runtime.Stop(cleanupCtx, containerID); err != nil {
		console.Warning(fmt.Sprintf("Stop failed, force removing: %v", err))
		_ = opts.Runtime.Remove(cleanupCtx, containerID)
	}

	console.Success("Sandbox stopped")
	return nil
}
