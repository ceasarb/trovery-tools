package cli

import (
	"fmt"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage org and repo policy configuration",
}

var policyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the merged policy (org + repo)",
	RunE:  runPolicyShow,
}

var policyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the merged policy for locked-field violations",
	RunE:  runPolicyValidate,
}

func init() {
	policyCmd.AddCommand(policyShowCmd)
	policyCmd.AddCommand(policyValidateCmd)
	rootCmd.AddCommand(policyCmd)
}

func runPolicyShow(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	org, err := config.LoadOrgPolicy()
	if err != nil {
		return fmt.Errorf("loading org policy: %w", err)
	}

	if org != nil {
		config.MergeOrgPolicy(cfg, org)
		console.Header("Merged Policy (org + repo)")
	} else {
		console.Header("Policy (repo only — no org policy found)")
		console.Dim(fmt.Sprintf("  Org policy path: %s", config.OrgPolicyPath()))
	}

	view := config.MergedConfigView(cfg, org)
	for k, v := range view {
		console.Dim(fmt.Sprintf("  %s: %s", k, v))
	}

	fmt.Println()
	console.Header("Secrets Patterns")
	for _, p := range cfg.Policies.Secrets.BlockPatterns {
		console.Dim(fmt.Sprintf("  - %s", p))
	}

	fmt.Println()
	console.Header("Filesystem Read-Only")
	for _, p := range cfg.Policies.Filesystem.ReadOnly {
		console.Dim(fmt.Sprintf("  - %s", p))
	}

	if len(cfg.Policies.Filesystem.NoWrite) > 0 {
		fmt.Println()
		console.Header("Filesystem No-Write")
		for _, p := range cfg.Policies.Filesystem.NoWrite {
			console.Dim(fmt.Sprintf("  - %s", p))
		}
	}

	return nil
}

func runPolicyValidate(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	org, err := config.LoadOrgPolicy()
	if err != nil {
		return fmt.Errorf("loading org policy: %w", err)
	}

	if org == nil {
		console.Success("No org policy found — repo policy is unrestricted.")
		return nil
	}

	violations := config.MergeOrgPolicy(cfg, org)
	if len(violations) == 0 {
		console.Success("Merged policy is valid — no locked-field violations.")
		return nil
	}

	console.Error(fmt.Sprintf("Found %d locked-field violation(s):", len(violations)))
	for _, v := range violations {
		console.Error(fmt.Sprintf("  - %s", v))
	}

	return fmt.Errorf("%d locked-field violation(s)", len(violations))
}
