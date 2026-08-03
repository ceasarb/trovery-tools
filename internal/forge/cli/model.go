package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ceasarb/trovery-tools/internal/forge/model"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Local model management — pull, list, remove",
	Long: console.HeaderStyle.Render("Model Commands") + "\n\n" +
		"Manage local AI models:\n" +
		"  pull    Pull a model from Ollama or HuggingFace\n" +
		"  list    List registered local models\n" +
		"  remove  Remove a local model",
}

var modelPullCmd = &cobra.Command{
	Use:   "pull <name>",
	Short: "Pull a model from Ollama or HuggingFace",
	Long: "Pull a model by name. Accepts Ollama names (mistral, llama3) or\n" +
		"HuggingFace refs (hf:mistralai/Mistral-7B-v0.1).",
	Args: cobra.ExactArgs(1),
	RunE: runModelPull,
}

func runModelPull(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Require Ollama
	status, err := model.RequireOllama()
	if err != nil {
		console.Error(err.Error())
		return err
	}
	console.Dim(fmt.Sprintf("  Ollama v%s at %s", status.Version, status.Endpoint))

	ollamaName := model.TranslateHFName(name)
	console.Header("Pulling: " + ollamaName)
	fmt.Println()

	// Progress display
	lastStatus := ""
	err = model.Pull(name, func(status string, completed, total int64) {
		if total > 0 {
			pct := float64(completed) / float64(total) * 100
			fmt.Fprintf(os.Stderr, "\r  %s: %.0f%%", status, pct)
		} else if status != lastStatus {
			if lastStatus != "" {
				fmt.Fprintln(os.Stderr)
			}
			fmt.Fprintf(os.Stderr, "  %s", status)
			lastStatus = status
		}
	})
	fmt.Fprintln(os.Stderr) // newline after progress

	if err != nil {
		console.Error(fmt.Sprintf("Pull failed: %v", err))
		return err
	}

	fmt.Println()
	console.Success("Model pulled and registered: " + ollamaName)
	return nil
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered local models",
	RunE:  runModelList,
}

func runModelList(cmd *cobra.Command, args []string) error {
	// Check Ollama status (non-fatal)
	status := model.DetectOllama()
	if !status.Available {
		console.Warning("Ollama is not running — showing registry data only")
		fmt.Println()
	}

	models, err := model.ListModels()
	if err != nil {
		console.Error(fmt.Sprintf("List models: %v", err))
		return err
	}

	if len(models) == 0 {
		console.Info("No models found. Pull one with: trove forge model pull <name>")
		return nil
	}

	console.Header("Local Models")
	fmt.Println()

	// Table header
	fmt.Printf("  %-30s %-14s %-10s %-12s %s\n",
		console.DimStyle.Render("NAME"),
		console.DimStyle.Render("SOURCE"),
		console.DimStyle.Render("SIZE"),
		console.DimStyle.Render("PULLED"),
		console.DimStyle.Render("LAST USED"),
	)
	fmt.Println()

	for _, m := range models {
		pulled := ""
		if !m.PulledAt.IsZero() {
			pulled = m.PulledAt.Format("2006-01-02")
		}
		lastUsed := "-"
		if !m.LastUsed.IsZero() {
			lastUsed = m.LastUsed.Format("2006-01-02")
		}

		fmt.Printf("  %-30s %-14s %-10s %-12s %s\n",
			m.Name, m.Source, m.Size, pulled, lastUsed)
	}

	return nil
}

var (
	modelRemoveForce bool
)

var modelRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a local model",
	Args:  cobra.ExactArgs(1),
	RunE:  runModelRemove,
}

func runModelRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Require Ollama
	if _, err := model.RequireOllama(); err != nil {
		console.Error(err.Error())
		return err
	}

	// Confirmation prompt unless --force
	if !modelRemoveForce {
		fmt.Printf("Remove model %s? [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			console.Dim("  Cancelled")
			return nil
		}
	}

	if err := model.Remove(name); err != nil {
		console.Error(fmt.Sprintf("Remove failed: %v", err))
		return err
	}

	console.Success("Model removed: " + name)
	return nil
}

func init() {
	modelRemoveCmd.Flags().BoolVarP(&modelRemoveForce, "force", "f", false, "Skip confirmation prompt")

	modelCmd.AddCommand(modelPullCmd)
	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelRemoveCmd)
}
