package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/skills"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/workspace"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage agent skills (create, test, attach, pack)",
}

// --- skill create ---

var skillCreateNamespace string

var skillCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new SKILL.md skill directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillCreate,
}

func runSkillCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace := skillCreateNamespace
	if namespace == "" {
		namespace = "local"
	}

	// Place skill in skills/ directory (workspace-aware)
	outputDir, err := workspace.DetectOutputDir("skill")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	dir := filepath.Join(outputDir, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("skill already exists: %s", dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	os.MkdirAll(filepath.Join(dir, "examples"), 0o755)

	skillMD := fmt.Sprintf(`---
name: %s
description: >
  Describe what this skill does and when to use it.
tags: []
metadata:
  demi.namespace: %s
  demi.version: 0.1.0
---

# %s

## Instructions

Describe the step-by-step instructions for this skill here.

The agent will receive these instructions when it activates this skill.
`, name, namespace, strings.ReplaceAll(name, "-", " "))

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		return err
	}

	console.Success(fmt.Sprintf("Created skill: %s", dir))
	console.Dim(fmt.Sprintf("  %s/SKILL.md", dir))
	console.Dim(fmt.Sprintf("  %s/scripts/", dir))
	console.Dim(fmt.Sprintf("  %s/examples/", dir))
	fmt.Println()
	console.Info("Next: edit SKILL.md, then run: demi forge agent skill test " + dir)

	return nil
}

// --- skill test ---

var skillTestCmd = &cobra.Command{
	Use:   "test <path>",
	Short: "Validate a SKILL.md's structure and frontmatter",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillTest,
}

func runSkillTest(cmd *cobra.Command, args []string) error {
	dir := args[0]

	skill, result := skills.ValidateDirectory(dir)
	if skill != nil {
		console.Header("Skill: " + skill.Identity())
		if skill.Version != "" {
			console.Dim(fmt.Sprintf("  Version: %s", skill.Version))
		}
		fmt.Println()
	}

	if result.Valid {
		console.Success("Validation passed")
	} else {
		console.Error("Validation failed")
	}

	output := skills.FormatResult(result)
	if output != "" {
		fmt.Println(output)
	}

	if !result.Valid {
		return fmt.Errorf("skill validation failed")
	}
	return nil
}

// --- skill attach ---

var skillAttachSkillPath string

var skillAttachCmd = &cobra.Command{
	Use:   "attach <agent-name> --skill <path>",
	Short: "Wire a skill to an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillAttach,
}

func runSkillAttach(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	// Resolve skill path to absolute for validation
	absSkillPath, err := filepath.Abs(skillAttachSkillPath)
	if err != nil {
		return err
	}

	// Validate the skill
	skill, err := skills.ParseSkill(absSkillPath)
	if err != nil {
		return err
	}

	// Check if already attached
	if cfg.Skills != nil {
		for _, ref := range cfg.Skills.Attached {
			existingAbs, _ := filepath.Abs(ref.Path)
			if existingAbs == absSkillPath {
				console.Warning(fmt.Sprintf("Skill already attached: %s", skill.Identity()))
				return nil
			}
		}
	}

	// Store path relative to agent directory
	relPath, err := filepath.Rel(agentDir, absSkillPath)
	if err != nil {
		relPath = absSkillPath
	}

	// Add to config
	if cfg.Skills == nil {
		cfg.Skills = &agentcfg.SkillsConfig{}
	}
	cfg.Skills.Attached = append(cfg.Skills.Attached, agentcfg.SkillRef{Path: relPath})

	if err := agentcfg.Save(agentDir, cfg); err != nil {
		return err
	}

	console.Success(fmt.Sprintf("Attached %s to agent %s", skill.Identity(), cfg.Name))
	console.Dim(fmt.Sprintf("  %s → %s", relPath, absSkillPath))
	return nil
}

// --- skill detach ---

var skillDetachName string

var skillDetachCmd = &cobra.Command{
	Use:   "detach <agent-name> --skill <name>",
	Short: "Remove a skill from an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillDetach,
}

func runSkillDetach(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	if cfg.Skills == nil || len(cfg.Skills.Attached) == 0 {
		console.Warning("No skills attached")
		return nil
	}

	found := false
	var remaining []agentcfg.SkillRef
	for _, ref := range cfg.Skills.Attached {
		// Match by path basename or full path
		if filepath.Base(ref.Path) == skillDetachName || ref.Path == skillDetachName {
			found = true
			continue
		}
		remaining = append(remaining, ref)
	}

	if !found {
		return fmt.Errorf("skill %q not found in agent %s", skillDetachName, cfg.Name)
	}

	cfg.Skills.Attached = remaining
	if err := agentcfg.Save(agentDir, cfg); err != nil {
		return err
	}

	console.Success(fmt.Sprintf("Detached %s from agent %s", skillDetachName, cfg.Name))
	return nil
}

// --- skill list ---

var skillListCmd = &cobra.Command{
	Use:   "list <agent-name>",
	Short: "Show skills attached to an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillList,
}

func runSkillList(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	console.Header("Skills: " + cfg.Name)
	fmt.Println()

	if cfg.Skills == nil || len(cfg.Skills.Attached) == 0 {
		console.Dim("  No skills attached")
		console.Dim("  Attach a skill: demi forge agent skill attach " + cfg.Name + " --skill <path>")
		return nil
	}

	// Check Vigil
	vigil := skills.DetectVigil()
	if vigil != nil {
		console.Dim("  Vigil governance: active")
	}

	for _, ref := range cfg.Skills.Attached {
		// Resolve relative paths against agent directory
		skillPath := ref.Path
		if !filepath.IsAbs(skillPath) {
			skillPath = filepath.Join(agentDir, skillPath)
		}
		skill, err := skills.ParseSkill(skillPath)
		if err != nil {
			console.Error(fmt.Sprintf("  ✗ %s — %v", ref.Path, err))
			continue
		}

		status := "✓"
		if vigil != nil {
			result := vigil.CheckPolicy(skill.Identity())
			if result.Status == "blocked" {
				status = "✗ BLOCKED"
			}
		}

		console.Dim(fmt.Sprintf("  %s %s — %s", status, skill.Identity(), skill.Description))
	}

	return nil
}

// --- skill pack ---

var skillPackCmd = &cobra.Command{
	Use:   "pack <path>",
	Short: "Create a distributable skill archive",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillPack,
}

func runSkillPack(cmd *cobra.Command, args []string) error {
	dir := args[0]

	console.Dim("  Validating skill...")
	archivePath, err := skills.Pack(dir)
	if err != nil {
		return err
	}

	console.Success(fmt.Sprintf("Packed: %s", archivePath))
	return nil
}

func init() {
	skillCreateCmd.Flags().StringVar(&skillCreateNamespace, "namespace", "local", "Skill namespace")
	skillAttachCmd.Flags().StringVar(&skillAttachSkillPath, "skill", "", "Path to skill directory")
	skillAttachCmd.MarkFlagRequired("skill")
	skillDetachCmd.Flags().StringVar(&skillDetachName, "skill", "", "Skill name or path to detach")
	skillDetachCmd.MarkFlagRequired("skill")

	skillCmd.AddCommand(skillCreateCmd)
	skillCmd.AddCommand(skillTestCmd)
	skillCmd.AddCommand(skillAttachCmd)
	skillCmd.AddCommand(skillDetachCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillPackCmd)

	agentCmd.AddCommand(skillCmd)
}
