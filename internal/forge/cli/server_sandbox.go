package cli

import (
	"fmt"
	"os"

	"github.com/ceasarb/trovery-tools/pkg/forge/server/sandbox"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/console"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/container"
	"github.com/spf13/cobra"
)

var (
	sandboxPolicy  string
	sandboxRuntime string
	sandboxPort    string
)

var serverSandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Run server in a sandboxed container",
	Long: "Build and run the current MCP server in an isolated container with\n" +
		"configurable security policies (strict, standard, permissive).\n\n" +
		"Must be run from inside a server directory (contains trove.toml).",
	RunE: runServerSandbox,
}

func runServerSandbox(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load server config
	cfg, err := config.LoadServerConfig(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("No trove.toml found: %v", err))
		console.Dim("  Run this command from inside a server directory")
		return err
	}

	console.Header("Sandbox: " + cfg.Server.Name)
	fmt.Println()

	// Resolve container runtime
	var rt container.Runtime
	if sandboxRuntime != "" {
		rt, err = container.RuntimeByName(sandboxRuntime)
		if err != nil {
			console.Error(err.Error())
			return err
		}
		if !rt.IsAvailable() {
			console.Error(fmt.Sprintf("%s is not available on this system", sandboxRuntime))
			return fmt.Errorf("%s not available", sandboxRuntime)
		}
	} else {
		rt, err = container.DetectRuntime()
		if err != nil {
			console.Error("No container runtime found")
			console.Dim("  Install Docker, Podman, or nerdctl to use sandbox mode")
			return err
		}
	}
	console.Dim(fmt.Sprintf("  Runtime: %s", rt.Name()))

	// Resolve security policy
	policy, err := sandbox.ResolvePolicy(sandboxPolicy)
	if err != nil {
		console.Error(err.Error())
		return err
	}
	console.Dim(fmt.Sprintf("  Policy: %s", policy.Name))
	fmt.Println()

	return sandbox.Run(sandbox.SandboxOpts{
		ServerDir: cwd,
		Config:    cfg,
		Policy:    policy,
		Runtime:   rt,
		Port:      sandboxPort,
	})
}

func init() {
	serverSandboxCmd.Flags().StringVar(&sandboxPolicy, "policy", "standard", "Security policy: strict, standard, permissive, or path to YAML file")
	serverSandboxCmd.Flags().StringVar(&sandboxRuntime, "runtime", "", "Container runtime override: docker, podman, or nerdctl")
	serverSandboxCmd.Flags().StringVar(&sandboxPort, "port", "", "Port mapping (e.g., 8080:8080)")

	serverCmd.AddCommand(serverSandboxCmd)
}
