package sandbox

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SecurityPolicy defines resource limits and network rules for a sandboxed container.
type SecurityPolicy struct {
	Name     string   `yaml:"name"`
	Network  bool     `yaml:"network"`
	Domains  []string `yaml:"domains"`
	ReadOnly bool     `yaml:"read_only"`
	MemoryMB int      `yaml:"memory_mb"`
	CPUs     float64  `yaml:"cpus"`
	PidsLimit int     `yaml:"pids_limit"`
}

var presets = map[string]SecurityPolicy{
	"strict": {
		Name:      "strict",
		Network:   false,
		ReadOnly:  true,
		MemoryMB:  256,
		CPUs:      0.5,
		PidsLimit: 50,
	},
	"standard": {
		Name:      "standard",
		Network:   true,
		Domains:   []string{},
		ReadOnly:  false,
		MemoryMB:  512,
		CPUs:      1.0,
		PidsLimit: 100,
	},
	"permissive": {
		Name:      "permissive",
		Network:   true,
		Domains:   []string{},
		ReadOnly:  false,
		MemoryMB:  1024,
		CPUs:      2.0,
		PidsLimit: 200,
	},
}

// GetPreset returns a built-in security policy by name.
func GetPreset(name string) (*SecurityPolicy, error) {
	p, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown policy preset: %s (expected strict, standard, or permissive)", name)
	}
	return &p, nil
}

// LoadPolicyFile reads a custom security policy from a YAML file.
func LoadPolicyFile(path string) (*SecurityPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	var p SecurityPolicy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}

	if p.Name == "" {
		p.Name = "custom"
	}

	return &p, nil
}

// ResolvePolicy returns a policy from either a preset name or a file path.
func ResolvePolicy(nameOrPath string) (*SecurityPolicy, error) {
	// Try preset first
	if p, err := GetPreset(nameOrPath); err == nil {
		return p, nil
	}

	// Try as file path
	if _, err := os.Stat(nameOrPath); err == nil {
		return LoadPolicyFile(nameOrPath)
	}

	return nil, fmt.Errorf("policy %q is not a preset (strict, standard, permissive) or a valid file path", nameOrPath)
}
