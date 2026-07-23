package cli

import (
	"fmt"
	"strings"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/deploy"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var (
	deployTarget string
	deployOutput string
)

var agentDeployCmd = &cobra.Command{
	Use:   "deploy [agent-name]",
	Short: "Generate production deployment artifacts",
	Long: console.HeaderStyle.Render("Agent Deploy") + "\n\n" +
		"Generate production-ready deployment artifacts from an agent's config.\n\n" +
		"Targets:\n" +
		"  docker           Hardened Dockerfile + docker-compose.yml\n" +
		"  kubernetes       Helm chart (deployment, service, HPA, NetworkPolicy)\n" +
		"  cloud-run        Terraform module (GCP Cloud Run + Secret Manager)\n" +
		"  container-apps   Bicep template (Azure Container Apps + Key Vault)\n" +
		"  github-actions   CI/CD workflows (validate, eval, deploy)\n" +
		"  all              Generate all targets",
	Args: cobra.ExactArgs(1),
	RunE: runAgentDeploy,
}

func runAgentDeploy(cmd *cobra.Command, args []string) error {
	if deployTarget == "" {
		console.Error("--target flag is required")
		printValidTargets()
		return fmt.Errorf("missing --target flag")
	}

	if !deploy.IsValidTarget(deployTarget) {
		console.Error(fmt.Sprintf("Unknown target: %s", deployTarget))
		printValidTargets()
		return fmt.Errorf("unknown target: %s", deployTarget)
	}

	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	console.Header("Deploying agent: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Target: %s", deployTarget))
	console.Dim(fmt.Sprintf("  Output: %s", deployOutput))
	fmt.Println()

	deployer := deploy.New(cfg, deploy.Target(deployTarget), deployOutput)
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

func printValidTargets() {
	targets := make([]string, len(deploy.ValidTargets()))
	for i, t := range deploy.ValidTargets() {
		targets[i] = string(t)
	}
	console.Dim("  Valid targets: " + strings.Join(targets, ", ") + ", all")
}

func init() {
	agentDeployCmd.Flags().StringVar(&deployTarget, "target", "", "Deployment target: docker, kubernetes, cloud-run, container-apps, github-actions, all (required)")
	agentDeployCmd.Flags().StringVar(&deployOutput, "output", "./deploy", "Output directory")
	agentCmd.AddCommand(agentDeployCmd)
}
