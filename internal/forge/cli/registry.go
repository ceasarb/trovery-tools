package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ceasarb/demigo-tools/internal/forge/server/registry"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Server registry — discover and publish MCP servers",
	Long: console.HeaderStyle.Render("Registry Commands") + "\n\n" +
		"Manage the local MCP server registry:\n" +
		"  search    Search for servers by name, tag, or category\n" +
		"  info      Show full details for a registered server\n" +
		"  publish   Register the current server in the local index",
}

var registrySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for servers in the local registry",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRegistrySearch,
}

func runRegistrySearch(cmd *cobra.Command, args []string) error {
	idx, err := loadRegistry()
	if err != nil {
		return err
	}

	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	results := idx.Search(query)

	if len(results) == 0 {
		if query != "" {
			console.Info(fmt.Sprintf("No servers found matching %q", query))
		} else {
			console.Info("Registry is empty. Run: demi forge server registry publish")
		}
		return nil
	}

	if query != "" {
		console.Header(fmt.Sprintf("Search results for %q (%d found)", query, len(results)))
	} else {
		console.Header(fmt.Sprintf("All registered servers (%d)", len(results)))
	}
	fmt.Println()

	for _, r := range results {
		e := r.Entry
		line := fmt.Sprintf("  %s", e.Name)
		if e.Description != "" {
			line += fmt.Sprintf(" — %s", e.Description)
		}
		console.Dim(line)

		var meta []string
		if len(e.Tags) > 0 {
			meta = append(meta, "tags: "+strings.Join(e.Tags, ", "))
		}
		if len(e.Categories) > 0 {
			meta = append(meta, "categories: "+strings.Join(e.Categories, ", "))
		}
		if e.Transport != "" {
			meta = append(meta, "transport: "+e.Transport)
		}
		if len(meta) > 0 {
			console.Dim("    " + strings.Join(meta, "  |  "))
		}
	}

	return nil
}

var registryInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show full details for a registered server",
	Args:  cobra.ExactArgs(1),
	RunE:  runRegistryInfo,
}

func runRegistryInfo(cmd *cobra.Command, args []string) error {
	idx, err := loadRegistry()
	if err != nil {
		return err
	}

	entry := idx.Info(args[0])
	if entry == nil {
		console.Error(fmt.Sprintf("Server not found: %s", args[0]))
		return fmt.Errorf("server not found: %s", args[0])
	}

	console.Header("Server: " + entry.Name)
	fmt.Println()

	if entry.Description != "" {
		console.Info("Description")
		console.Dim("  " + entry.Description)
		fmt.Println()
	}

	console.Info("Transport")
	console.Dim("  " + entry.Transport)
	fmt.Println()

	if entry.Command != "" {
		console.Info("Command")
		console.Dim("  " + entry.Command)
		fmt.Println()
	}

	console.Info("Path")
	console.Dim("  " + entry.Path)
	fmt.Println()

	if len(entry.Tags) > 0 {
		console.Info("Tags")
		console.Dim("  " + strings.Join(entry.Tags, ", "))
		fmt.Println()
	}

	if len(entry.Categories) > 0 {
		console.Info("Categories")
		console.Dim("  " + strings.Join(entry.Categories, ", "))
		fmt.Println()
	}

	if entry.Author != "" {
		console.Info("Author")
		console.Dim("  " + entry.Author)
		fmt.Println()
	}

	if entry.License != "" {
		console.Info("License")
		console.Dim("  " + entry.License)
		fmt.Println()
	}

	if entry.MinMCPVersion != "" {
		console.Info("Min MCP Version")
		console.Dim("  " + entry.MinMCPVersion)
		fmt.Println()
	}

	if entry.Homepage != "" {
		console.Info("Homepage")
		console.Dim("  " + entry.Homepage)
		fmt.Println()
	}

	console.Info("Published")
	console.Dim("  " + entry.PublishedAt)

	return nil
}

var registryPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Register the current server in the local index",
	RunE:  runRegistryPublish,
}

func runRegistryPublish(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	idx, err := loadRegistry()
	if err != nil {
		return err
	}

	entry, err := idx.Publish(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("Publish failed: %v", err))
		return err
	}

	if err := idx.Save(); err != nil {
		console.Error(fmt.Sprintf("Save registry: %v", err))
		return err
	}

	console.Success(fmt.Sprintf("Published %s to local registry", entry.Name))
	fmt.Println()
	console.Dim("  Name:      " + entry.Name)
	if entry.Description != "" {
		console.Dim("  Desc:      " + entry.Description)
	}
	console.Dim("  Transport: " + entry.Transport)
	console.Dim("  Path:      " + entry.Path)
	if len(entry.Tags) > 0 {
		console.Dim("  Tags:      " + strings.Join(entry.Tags, ", "))
	}

	return nil
}

func loadRegistry() (*registry.Index, error) {
	path, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	return registry.Load(path)
}

func init() {
	registryCmd.AddCommand(registrySearchCmd)
	registryCmd.AddCommand(registryInfoCmd)
	registryCmd.AddCommand(registryPublishCmd)
	serverCmd.AddCommand(registryCmd)
}
