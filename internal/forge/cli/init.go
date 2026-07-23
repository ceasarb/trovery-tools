package cli

import (
	"fmt"
	"os/exec"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/workspace"
	"github.com/spf13/cobra"
)

var noServers bool

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a new Demigo Forge workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	name := args[0]

	console.Header("Creating workspace: " + name)

	path, err := workspace.Init(name, noServers)
	if err != nil {
		console.Error(fmt.Sprintf("Failed to create workspace: %v", err))
		return err
	}

	// Initialize git repo
	gitCmd := exec.Command("git", "init", path)
	if err := gitCmd.Run(); err != nil {
		console.Warning("Could not initialize git repository")
	}

	// Print summary
	fmt.Println()
	console.Success("Workspace created at " + path)
	fmt.Println()

	console.Dim("  " + name + "/")
	console.Dim("  ├── .demi/forge.yaml")
	console.Dim("  ├── agents/")
	if !noServers {
		console.Dim("  ├── servers/")
	}
	console.Dim("  ├── README.md")
	console.Dim("  └── .gitignore")

	fmt.Println()
	console.Info("Next steps:")
	console.Dim("  cd " + name)
	if !noServers {
		console.Dim("  demi forge server create")
	}
	console.Dim("  demi forge agent create")

	return nil
}

func init() {
	initCmd.Flags().BoolVar(&noServers, "no-servers", false, "Create agent-only workspace (no servers/ directory)")
	rootCmd.AddCommand(initCmd)
}
