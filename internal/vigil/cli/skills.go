package cli

import (
	"fmt"

	"github.com/ceasarb/trovery-tools/internal/vigil/policy"
	"github.com/ceasarb/trovery-tools/internal/vigil/shared/console"
	"github.com/ceasarb/trovery-tools/internal/vigil/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skill governance",
}

var skillsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover skills in configured paths",
	RunE:  runSkillsScan,
}

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate skills against policies",
	RunE:  runSkillsCheck,
}

func init() {
	skillsCmd.AddCommand(skillsScanCmd)
	skillsCmd.AddCommand(skillsCheckCmd)
	rootCmd.AddCommand(skillsCmd)
}

func runSkillsScan(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	paths := cfg.Skills.ScanPaths
	if len(paths) == 0 {
		console.Dim("No skill scan paths configured in .trove/vigil.yaml.")
		return nil
	}

	scanner := skills.NewSkillScanner(paths)
	discovered, err := scanner.Scan()
	if err != nil {
		return err
	}

	if len(discovered) == 0 {
		console.Dim("No skills found.")
		return nil
	}

	console.Header(fmt.Sprintf("Discovered %d skill(s)", len(discovered)))
	headers := []string{"Name", "Namespace", "Version", "Description", "Path"}
	var rows [][]string
	for _, s := range discovered {
		rows = append(rows, []string{
			s.Name,
			s.Namespace,
			s.Version,
			truncate(s.Description, 40),
			s.Path,
		})
	}
	console.Table(headers, rows)

	return nil
}

func runSkillsCheck(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	paths := cfg.Skills.ScanPaths
	if len(paths) == 0 {
		console.Dim("No skill scan paths configured.")
		return nil
	}

	scanner := skills.NewSkillScanner(paths)
	discovered, err := scanner.Scan()
	if err != nil {
		return err
	}

	if len(discovered) == 0 {
		console.Dim("No skills found.")
		return nil
	}

	engine := policy.NewSkillPolicyEngine(cfg.Skills)
	violations := engine.CheckSkills(discovered)

	if len(violations) == 0 {
		console.Success(fmt.Sprintf("All %d skill(s) pass policy checks", len(discovered)))
		return nil
	}

	for _, v := range violations {
		if v.Severity == "error" {
			console.Error(fmt.Sprintf("[%s] %s", v.Rule, v.Message))
		} else {
			console.Warning(fmt.Sprintf("[%s] %s", v.Rule, v.Message))
		}
	}

	return nil
}
