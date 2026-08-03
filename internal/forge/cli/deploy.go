package cli

import (
	"fmt"
	"os"
	"strings"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/deploy"
	sharedconfig "github.com/ceasarb/trovery-tools/internal/forge/shared/config"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var (
	serverDeployTarget string
	serverDeployOutput string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Generate production deployment artifacts for MCP servers",
	Long: console.HeaderStyle.Render("Deploy") + "\n\n" +
		"Generate production-ready deployment artifacts for MCP servers.\n" +
		"Uses the same targets as 'trove forge agent deploy'.\n\n" +
		"Targets:\n" +
		"  docker           Hardened Dockerfile + docker-compose.yml\n" +
		"  kubernetes       Helm chart\n" +
		"  cloud-run        Terraform module (GCP)\n" +
		"  container-apps   Bicep template (Azure)\n" +
		"  github-actions   CI/CD workflows\n" +
		"  all              Generate all targets",
	RunE: runServerDeploy,
}

func runServerDeploy(cmd *cobra.Command, args []string) error {
	if serverDeployTarget == "" {
		console.Error("--target flag is required")
		targets := make([]string, len(deploy.ValidTargets()))
		for i, t := range deploy.ValidTargets() {
			targets[i] = string(t)
		}
		console.Dim("  Valid targets: " + strings.Join(targets, ", ") + ", all")
		return fmt.Errorf("missing --target flag")
	}

	if !deploy.IsValidTarget(serverDeployTarget) {
		console.Error(fmt.Sprintf("Unknown target: %s", serverDeployTarget))
		return fmt.Errorf("unknown target: %s", serverDeployTarget)
	}

	// For server deploy, create a minimal agent config from the server's context.
	// The deploy package generates infrastructure artifacts that work for both.
	cwd, err := resolveServerDir()
	if err != nil {
		return err
	}

	serverCfg, err := loadServerConfig(cwd)
	if err != nil {
		return err
	}

	// Build a synthetic agent config representing the server for deployment.
	cfg := &agentcfg.AgentConfig{
		Name: serverCfg.Name,
	}

	console.Header("Deploying server: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Target: %s", serverDeployTarget))
	console.Dim(fmt.Sprintf("  Output: %s", serverDeployOutput))
	fmt.Println()

	deployer := deploy.New(cfg, deploy.Target(serverDeployTarget), serverDeployOutput)
	results, err := deployer.Deploy()
	if err != nil {
		console.Error(fmt.Sprintf("Deploy failed: %v", err))
		return err
	}

	for _, result := range results {
		console.Success(fmt.Sprintf("[%s] Generated %d file(s):", result.Target, len(result.Files)))
		for _, f := range result.Files {
			console.Dim(fmt.Sprintf("  %s — %s", f.Path, f.Description))
		}
		fmt.Println()

		if len(result.NextSteps) > 0 {
			console.Info("Next steps:")
			for _, step := range result.NextSteps {
				console.Dim("  " + step)
			}
			fmt.Println()
		}
	}

	return nil
}

// resolveServerDir finds the current server directory.
func resolveServerDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// serverConfigInfo holds minimal server metadata for deploy.
type serverConfigInfo struct {
	Name string
}

// loadServerConfig reads the server name from trove.toml in the given directory.
func loadServerConfig(dir string) (*serverConfigInfo, error) {
	cfg, err := sharedconfig.LoadServerConfig(dir)
	if err != nil {
		return nil, fmt.Errorf("not a valid server directory (missing trove.toml): %w", err)
	}
	return &serverConfigInfo{Name: cfg.Server.Name}, nil
}

func init() {
	deployCmd.Flags().StringVar(&serverDeployTarget, "target", "", "Deployment target (required)")
	deployCmd.Flags().StringVar(&serverDeployOutput, "output", "./deploy", "Output directory")
	serverCmd.AddCommand(deployCmd)
}
