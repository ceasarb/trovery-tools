package cli

import (
	"fmt"
	"os"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "demi-forge",
	Short: "Demigo Forge — forge your AI from prototype to production",
	Long: console.HeaderStyle.Render("Demigo Forge") + "\n" +
		console.DimStyle.Render("Forge your AI — from prototype to production") + "\n\n" +
		"Scaffold, test, and deploy MCP servers and AI agents.\n" +
		"Get started: demi forge init my-project",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, console.ErrorStyle.Render("Error: "+err.Error()))
		os.Exit(1)
	}
	return nil
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(
		console.HeaderStyle.Render("Demigo Forge") + " " +
			console.DimStyle.Render("v{{.Version}}") + "\n",
	)

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(dashboardCmd)
}
