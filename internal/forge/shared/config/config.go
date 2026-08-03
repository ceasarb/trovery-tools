package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ServerConfig represents server configuration for an MCP server.
type ServerConfig struct {
	Server   ServerSection   `toml:"server"`
	Testing  TestingSection  `toml:"testing"`
	Registry RegistrySection `toml:"registry"`
}

type ServerSection struct {
	Name      string   `toml:"name"`
	Entry     string   `toml:"entry"`
	Command   string   `toml:"command"`
	Transport string   `toml:"transport"`
	Port      int      `toml:"port,omitempty"`
	Env       []string `toml:"env,omitempty"` // required env var names (e.g. ["NHL_API_KEY", "ODDS_API_KEY"])
}

type TestingSection struct {
	Fixtures string `toml:"fixtures"`
}

// RegistrySection holds metadata for server discovery and indexing.
type RegistrySection struct {
	Description   string   `toml:"description,omitempty"`
	Tags          []string `toml:"tags,omitempty"`
	Categories    []string `toml:"categories,omitempty"`
	Author        string   `toml:"author,omitempty"`
	License       string   `toml:"license,omitempty"`
	MinMCPVersion string   `toml:"min_mcp_version,omitempty"`
	Homepage      string   `toml:"homepage,omitempty"`
}

// pyprojectFile is the top-level pyproject.toml structure.
type pyprojectFile struct {
	Project pyprojectProject `toml:"project"`
	Tool    pyprojectTool    `toml:"tool"`
}

type pyprojectProject struct {
	Name string `toml:"name"`
}

type pyprojectTool struct {
	Trove *pyprojectServer `toml:"trove"`
	// Legacy sections, newest first. Each names a prior brand generation:
	// Demigo, then Ajentis, then HatchDX. Read but never written.
	Demi *pyprojectServer `toml:"demi"`
	Ajnt *pyprojectServer `toml:"ajnt"`
	Hdx  *pyprojectServer `toml:"hdx"`
}

type pyprojectServer struct {
	ServerModule  string `toml:"server_module"`
	ServerCommand string `toml:"server_command"`
	FixturesDir   string `toml:"fixtures_dir"`
	Transport     string `toml:"transport"`
	Port          int    `toml:"port"`
}

// LoadServerConfig reads server config from the given directory.
// It tries these sources in order:
//  1. trove.toml (or legacy demi.toml, ajnt.toml)
//  2. pyproject.toml [tool.trove] (or legacy [tool.demi], [tool.ajnt])
//  3. pyproject.toml [tool.hdx] (backward compat)
func LoadServerConfig(dir string) (*ServerConfig, error) {
	// Try trove.toml first, then the legacy names from earlier rebrands
	for _, name := range []string{"trove.toml", "demi.toml", "ajnt.toml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var cfg ServerConfig
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		return &cfg, nil
	}

	// Try pyproject.toml
	pyPath := filepath.Join(dir, "pyproject.toml")
	data, err := os.ReadFile(pyPath)
	if err != nil {
		return nil, fmt.Errorf("no trove.toml or pyproject.toml found in %s", dir)
	}

	var pyproject pyprojectFile
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return nil, fmt.Errorf("parse pyproject.toml: %w", err)
	}

	// Prefer [tool.trove], then each legacy brand section newest-first.
	toolCfg := pyproject.Tool.Trove
	for _, legacy := range []*pyprojectServer{pyproject.Tool.Demi, pyproject.Tool.Ajnt, pyproject.Tool.Hdx} {
		if toolCfg != nil {
			break
		}
		toolCfg = legacy
	}
	if toolCfg == nil {
		return nil, fmt.Errorf("pyproject.toml has no [tool.trove], [tool.demi], [tool.ajnt], or [tool.hdx] section")
	}

	transport := toolCfg.Transport
	if transport == "" {
		transport = "stdio"
	}

	name := pyproject.Project.Name
	if name == "" {
		name = filepath.Base(dir)
	}

	return &ServerConfig{
		Server: ServerSection{
			Name:      name,
			Entry:     toolCfg.ServerModule,
			Command:   toolCfg.ServerCommand,
			Transport: transport,
			Port:      toolCfg.Port,
		},
		Testing: TestingSection{
			Fixtures: toolCfg.FixturesDir,
		},
	}, nil
}
