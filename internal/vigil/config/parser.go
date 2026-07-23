package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFileName = ".demi/vigil.yaml"

// ErrConfigNotFound indicates no .demi/vigil.yaml was found.
var ErrConfigNotFound = errors.New("no .demi/vigil.yaml found — run `demi vigil init` to create one")

// ConfigValidationError contains one or more validation issues.
type ConfigValidationError struct {
	Issues []string
}

func (e *ConfigValidationError) Error() string {
	return "invalid .demi/vigil.yaml:\n  " + strings.Join(e.Issues, "\n  ")
}

// FindConfig searches the current directory and parents for .demi/vigil.yaml.
// Returns the absolute path to the config file.
func FindConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, configFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrConfigNotFound
}

// LoadConfig loads and validates a .demi/vigil.yaml file.
// If path is empty, it searches upward from cwd.
func LoadConfig(path string) (*VigilConfig, error) {
	if path == "" {
		found, err := FindConfig()
		if err != nil {
			return nil, err
		}
		path = found
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *VigilConfig) error {
	var issues []string

	if cfg.Name == "" {
		issues = append(issues, "name is required")
	}

	// Validate regex patterns compile.
	for _, pattern := range cfg.Policies.Secrets.BlockPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			issues = append(issues, fmt.Sprintf("invalid regex pattern %q: %v", pattern, err))
		}
	}

	// Validate tool configs.
	for name, tc := range cfg.Tools {
		if name == "forge-agent" && tc.Enabled && tc.AgentPath == "" {
			issues = append(issues, "forge-agent requires agent_path when enabled")
		}
	}

	if len(issues) > 0 {
		return &ConfigValidationError{Issues: issues}
	}

	return nil
}

// ConfigDir returns the directory containing the config file.
func ConfigDir(configPath string) string {
	return filepath.Dir(configPath)
}
