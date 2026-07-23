package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a Vigil workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInit,
}

var (
	initTemplate string
	initForce    bool
	initCI       bool
)

func init() {
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "default", "Config template: default, python, typescript")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing .demi/vigil.yaml")
	initCmd.Flags().BoolVar(&initCI, "ci", false, "Generate GitHub Actions workflow")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine project name.
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		name = filepath.Base(cwd)
	}

	configPath := ".demi/vigil.yaml"

	// Check if config already exists.
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf(".demi/vigil.yaml already exists — use --force to overwrite")
	}

	// Render template.
	content, err := config.RenderTemplate(initTemplate, config.TemplateData{Name: name})
	if err != nil {
		return err
	}

	// Write .demi/vigil.yaml (create the .demi/ dir first — WriteFile won't).
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("creating .demi/ directory: %w", err)
	}
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("writing .demi/vigil.yaml: %w", err)
	}
	console.Success("Created .demi/vigil.yaml")

	// Create .demi/vigil/sessions/ directory.
	sessionsDir := filepath.Join(".demi/vigil", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("creating sessions directory: %w", err)
	}
	console.Success("Created .demi/vigil/sessions/")

	// Append to .gitignore.
	if err := appendGitignore(); err != nil {
		console.Warning(fmt.Sprintf("Could not update .gitignore: %v", err))
	} else {
		console.Success("Updated .gitignore")
	}

	// Generate CI workflow if requested.
	if initCI {
		if err := generateCIWorkflow(); err != nil {
			return err
		}
		console.Success("Created .github/workflows/vigil-audit.yaml")
	}

	fmt.Println()
	console.Dim(fmt.Sprintf("  Project: %s", name))
	console.Dim(fmt.Sprintf("  Template: %s", initTemplate))
	fmt.Println()
	console.Dim("  Next: demi vigil start")

	return nil
}

func appendGitignore() error {
	const marker = ".demi/vigil/"
	path := ".gitignore"

	// Read existing content.
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already present.
	if strings.Contains(string(existing), marker) {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := "\n# Demigo Vigil session data\n.demi/vigil/\n"
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		entry = "\n" + entry
	}

	_, err = f.WriteString(entry)
	return err
}

func generateCIWorkflow() error {
	dir := filepath.Join(".github", "workflows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating workflows directory: %w", err)
	}

	workflow := `name: Demigo Vigil Audit
on: [pull_request]

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Install Vigil
        run: |
          curl -sSfL https://github.com/ceasarb/demigo-tools/releases/latest/download/demi-vigil-linux-amd64 -o /usr/local/bin/demi-vigil
          chmod +x /usr/local/bin/demi-vigil

      - name: Audit PR changes
        run: |
          demi-vigil audit --ci \
            --base-ref ${{ github.event.pull_request.base.sha }} \
            --head-ref ${{ github.event.pull_request.head.sha }} \
            --format sarif \
            > vigil-results.sarif

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: vigil-results.sarif
`

	path := filepath.Join(dir, "vigil-audit.yaml")
	return os.WriteFile(path, []byte(workflow), 0644)
}
