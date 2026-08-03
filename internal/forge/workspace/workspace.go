package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

const MarkerFile = ".trove/forge.yaml"

// Workspace represents a detected workspace.
type Workspace struct {
	Root string
	Name string
}

// Find walks up from dir looking for the .trove/forge.yaml workspace marker.
// Returns nil if no workspace is found.
func Find(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	for {
		marker := filepath.Join(abs, MarkerFile)
		if _, err := os.Stat(marker); err == nil {
			name := filepath.Base(abs)
			return &Workspace{Root: abs, Name: name}, nil
		}

		parent := filepath.Dir(abs)
		if parent == abs {
			return nil, nil // reached filesystem root
		}
		abs = parent
	}
}

// ServersDir returns the servers directory path for the workspace.
func (w *Workspace) ServersDir() string {
	return filepath.Join(w.Root, "servers")
}

// AgentsDir returns the agents directory path for the workspace.
func (w *Workspace) AgentsDir() string {
	return filepath.Join(w.Root, "agents")
}

// SkillsDir returns the skills directory path for the workspace.
func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.Root, "skills")
}

// DetectOutputDir resolves where to place a new server or agent.
// If inside a workspace, returns the appropriate subdirectory.
// Otherwise, returns the current working directory.
func DetectOutputDir(kind string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	ws, err := Find(cwd)
	if err != nil {
		return "", err
	}

	if ws == nil {
		return cwd, nil
	}

	switch kind {
	case "server":
		return ws.ServersDir(), nil
	case "agent":
		return ws.AgentsDir(), nil
	case "skill":
		return ws.SkillsDir(), nil
	default:
		return cwd, nil
	}
}

// Init creates a new workspace at the given path.
func Init(name string, noServers bool) (string, error) {
	dir, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	// Write workspace marker under .trove/ (create the dir first — WriteFile won't)
	marker := filepath.Join(dir, MarkerFile)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", fmt.Errorf("create .trove/: %w", err)
	}
	content := fmt.Sprintf("workspace:\n  name: %q\n", filepath.Base(dir))
	if err := os.WriteFile(marker, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", MarkerFile, err)
	}

	// Create agents directory
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		return "", fmt.Errorf("create agents/: %w", err)
	}

	// Create servers directory unless --no-servers
	if !noServers {
		if err := os.MkdirAll(filepath.Join(dir, "servers"), 0o755); err != nil {
			return "", fmt.Errorf("create servers/: %w", err)
		}
	}

	// Write README
	readme := fmt.Sprintf("# %s\n\nAn [Trovery Forge](https://github.com/ceasarb/trovery-tools) workspace.\n", filepath.Base(dir))
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return "", fmt.Errorf("write README.md: %w", err)
	}

	// Write .gitignore
	gitignore := ".trove/forge/\n.env\n*.db\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return "", fmt.Errorf("write .gitignore: %w", err)
	}

	return dir, nil
}
