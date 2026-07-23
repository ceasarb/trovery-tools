package cli

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new governed session",
	RunE:  runStart,
}

var (
	startDryRun bool
	startForce  bool
	startTool   string
)

func init() {
	startCmd.Flags().BoolVar(&startDryRun, "dry-run", false, "Validate config without starting")
	startCmd.Flags().BoolVar(&startForce, "force", false, "Force-stop any stale session first")
	startCmd.Flags().StringVarP(&startTool, "tool", "t", "", "Pre-select a tool")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	// Validate tool if specified.
	if startTool != "" {
		tc, ok := cfg.Tools[startTool]
		if !ok {
			return fmt.Errorf("unknown tool %q — not defined in .demi/vigil.yaml", startTool)
		}
		if !tc.Enabled {
			reason := "no reason given"
			if tc.Reason != "" {
				reason = tc.Reason
			}
			return fmt.Errorf("tool %q is disabled: %s", startTool, reason)
		}
	}

	if startDryRun {
		console.Success("Config is valid.")
		console.Dim(fmt.Sprintf("  Project: %s", cfg.Name))

		var tools []string
		for name, tc := range cfg.Tools {
			if tc.Enabled {
				tools = append(tools, name)
			}
		}
		if len(tools) > 0 {
			console.Dim(fmt.Sprintf("  Enabled tools: %s", joinTools(tools)))
		}
		return nil
	}

	mgr := session.NewSessionManager(cfg, projectRoot)

	// Handle force-stop.
	if startForce {
		if old, _ := mgr.ForceStop(); old != nil {
			console.Warning(fmt.Sprintf("Force-stopped stale session %s", old.ID))
		}
	}

	s, err := mgr.Start()
	if err != nil {
		return err
	}

	console.Header(fmt.Sprintf("Session started: %s", s.ID))
	if s.GitSnapshot != nil {
		console.Dim(fmt.Sprintf("  Branch: %s", s.GitSnapshot.Branch))
		console.Dim(fmt.Sprintf("  HEAD: %s", truncate(s.GitSnapshot.HeadSHA, 8)))
	}

	var tools []string
	for name, tc := range cfg.Tools {
		if tc.Enabled {
			tools = append(tools, name)
		}
	}
	if len(tools) > 0 {
		console.Dim(fmt.Sprintf("  Tools: %s", joinTools(tools)))
	}

	fmt.Println()
	console.Dim("Run demi vigil run <tool> to launch a tool.")

	return nil
}

// loadConfigFromCwd finds and loads the config, returning the config and project root.
func loadConfigFromCwd() (*config.VigilConfig, string, error) {
	configPath, err := config.FindConfig()
	if err != nil {
		return nil, "", err
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, "", err
	}

	return cfg, config.ConfigDir(configPath), nil
}

func joinTools(tools []string) string {
	if len(tools) == 0 {
		return "none"
	}
	result := tools[0]
	for _, t := range tools[1:] {
		result += ", " + t
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
