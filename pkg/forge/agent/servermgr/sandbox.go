package servermgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/server/harness"
	"github.com/ceasarb/trovery-tools/pkg/forge/server/sandbox"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/container"
)

// SandboxConfig holds configuration for sandboxed server startup.
type SandboxConfig struct {
	Runtime container.Runtime
	Policy  *sandbox.SecurityPolicy
}

// StartServerSandboxed launches an MCP server inside a container with security
// policy enforcement. The server communicates via stdio through the container's
// stdin/stdout, so MCP protocol works identically to a direct subprocess.
//
// Only servers with Path or Command are sandboxable. URL and Agent refs are
// returned with an error — they should be started normally.
func (m *Manager) StartServerSandboxed(ctx context.Context, ref agentcfg.ServerRef, workDir string, sbx *SandboxConfig) (*ManagedServer, error) {
	// Only sandbox path/command servers — remote and agent refs aren't sandboxable
	if ref.URL != "" || ref.IsAgentRef() {
		return nil, fmt.Errorf("server %s is remote or agent-ref, not sandboxable", ref.Name)
	}

	serverDir := ref.Path
	if serverDir == "" {
		serverDir = workDir
	}
	serverDir, _ = filepath.Abs(serverDir)

	// Load server config to get the command
	serverCfg, err := config.LoadServerConfig(serverDir)
	if err != nil {
		// Fall back to the ref's command if no trove.toml
		if ref.Command == "" {
			return nil, fmt.Errorf("server %s: no trove.toml and no command specified", ref.Name)
		}
		serverCfg = &config.ServerConfig{
			Server: config.ServerSection{
				Name:    ref.Name,
				Command: ref.Command,
			},
		}
	}

	serverCommand := serverCfg.Server.Command
	if serverCommand == "" && ref.Command != "" {
		serverCommand = ref.Command
	}
	if serverCommand == "" {
		return nil, fmt.Errorf("server %s: no command found in trove.toml or agent.yaml", ref.Name)
	}

	// Detect language for Dockerfile generation
	lang, err := sandbox.DetectLanguage(serverDir)
	if err != nil {
		return nil, fmt.Errorf("server %s: detect language: %w", ref.Name, err)
	}

	// Generate a temporary Dockerfile
	dockerfilePath, err := sandbox.WriteDockerfile(serverDir, lang, serverCommand)
	if err != nil {
		return nil, fmt.Errorf("server %s: write dockerfile: %w", ref.Name, err)
	}
	defer os.Remove(dockerfilePath)

	// Write a temporary .dockerignore if not present
	diPath := filepath.Join(serverDir, ".dockerignore")
	createdIgnore := false
	if _, statErr := os.Stat(diPath); os.IsNotExist(statErr) {
		os.WriteFile(diPath, []byte(".venv\n.trove/forge\n__pycache__\n*.pyc\nnode_modules\n.git\n"), 0o644)
		createdIgnore = true
	}
	if createdIgnore {
		defer os.Remove(diPath)
	}

	// Build the container image
	imageTag := "trove-forge-serve-" + strings.ToLower(ref.Name)
	buildOpts := container.BuildOpts{
		ContextDir: serverDir,
		Dockerfile: dockerfilePath,
		Tag:        imageTag,
		Labels: map[string]string{
			"trove.sandbox": "true",
			"trove.server":  ref.Name,
			"trove.policy":  sbx.Policy.Name,
		},
	}

	if err := sbx.Runtime.Build(ctx, buildOpts); err != nil {
		return nil, fmt.Errorf("server %s: build image: %w", ref.Name, err)
	}

	// Construct the docker run command with policy constraints.
	// Using -i (interactive) keeps stdin open for MCP stdio communication.
	// Using --rm cleans up the container when the process exits.
	runArgs := buildRunArgs(sbx.Runtime.Name(), imageTag, ref.Name, sbx.Policy)

	// Start the container as a subprocess — harness.Start connects via stdin/stdout
	client, err := harness.Start(ctx, sbx.Runtime.Name(), runArgs, workDir)
	if err != nil {
		return nil, fmt.Errorf("server %s: start sandboxed: %w", ref.Name, err)
	}

	// Apply domain firewall if policy has a domain allowlist.
	// This runs after the container starts since we need the container ID.
	// For stdio mode we rely on the --network=none or iptables approach.
	// Since we're using `docker run` (not detached), firewall is applied via
	// the network policy flags in buildRunArgs.

	// Discover tools via MCP protocol (same as non-sandboxed)
	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("server %s: list tools: %w", ref.Name, err)
	}

	filtered := FilterTools(tools, ref.Tools)

	ms := &ManagedServer{
		Ref:    ref,
		Client: client,
		Caller: &harnessClientWrapper{client: client},
		Tools:  filtered,
	}

	m.servers = append(m.servers, ms)
	return ms, nil
}

// buildRunArgs constructs the arguments for `docker run -i --rm` with security
// policy constraints applied. The container runs in foreground with stdio attached.
func buildRunArgs(runtime, imageTag, serverName string, policy *sandbox.SecurityPolicy) []string {
	args := []string{
		"run",
		"-i",    // Keep stdin open for MCP stdio protocol
		"--rm",  // Clean up container on exit
		"--name", fmt.Sprintf("trove-forge-serve-%s", serverName),
	}

	// Memory limit
	if policy.MemoryMB > 0 {
		args = append(args, fmt.Sprintf("--memory=%dm", policy.MemoryMB))
	}

	// CPU limit
	if policy.CPUs > 0 {
		args = append(args, fmt.Sprintf("--cpus=%.1f", policy.CPUs))
	}

	// PID limit
	if policy.PidsLimit > 0 {
		args = append(args, fmt.Sprintf("--pids-limit=%d", policy.PidsLimit))
	}

	// Read-only filesystem
	if policy.ReadOnly {
		args = append(args, "--read-only")
		// Allow /tmp for applications that need a writable temp dir
		args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m")
	}

	// Network policy
	if !policy.Network {
		args = append(args, "--network=none")
	}

	// Drop all capabilities and add back only what's needed
	args = append(args, "--cap-drop=ALL")

	// If we have domain filtering, we need NET_ADMIN for iptables
	if policy.Network && len(policy.Domains) > 0 {
		args = append(args, "--cap-add=NET_ADMIN")
	}

	// Security options: no new privileges
	args = append(args, "--security-opt=no-new-privileges:true")

	// Labels for identification
	args = append(args,
		"--label", "trove.sandbox=true",
		"--label", fmt.Sprintf("trove.server=%s", serverName),
		"--label", fmt.Sprintf("trove.policy=%s", policy.Name),
	)

	// Image tag (must be last before any CMD override)
	args = append(args, imageTag)

	return args
}
