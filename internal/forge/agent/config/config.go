package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var validAgentName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// AgentConfig represents an agent.yaml file.
type AgentConfig struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description,omitempty"`
	Type         string              `yaml:"type,omitempty"` // "agent" (default) or "orchestrator"
	Model        ModelConfig         `yaml:"model"`
	System       string              `yaml:"system_prompt"`
	Servers      []ServerRef         `yaml:"servers,omitempty"`
	Settings     AgentSettings       `yaml:"settings,omitempty"`
	OTel         *OTelConfig         `yaml:"otel,omitempty"`
	Security     *SecurityConfig     `yaml:"security,omitempty"`
	Skills       *SkillsConfig       `yaml:"skills,omitempty"`
	Expose       *ExposeConfig       `yaml:"expose,omitempty"`
	Orchestrator *OrchestratorConfig `yaml:"orchestrator,omitempty"`
	Palette      *PaletteConfig      `yaml:"palette,omitempty"`
}

// PaletteConfig enables Palette UI rendering for the agent.
type PaletteConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Port          int    `yaml:"port,omitempty"`           // default: 4321
	AutoOpen      bool   `yaml:"auto_open,omitempty"`      // default: true
	PromptTimeout string `yaml:"prompt_timeout,omitempty"` // default: "5m", "0" disables
	Theme         string `yaml:"theme,omitempty"`          // path to palette theme JSON
}

// EffectivePort returns the configured port or the default (4321).
func (c *PaletteConfig) EffectivePort() int {
	if c == nil || c.Port == 0 {
		return 4321
	}
	return c.Port
}

// PaletteEnabled returns whether Palette is configured and enabled.
func (c *AgentConfig) PaletteEnabled() bool {
	return c.Palette != nil && c.Palette.Enabled
}

// SecurityConfig controls agent-level security features.
type SecurityConfig struct {
	SanitizeInput    *bool `yaml:"sanitize_input,omitempty"`     // default: true
	DetectInjection  *bool `yaml:"detect_injection,omitempty"`   // default: true
	FenceToolResults *bool `yaml:"fence_tool_results,omitempty"` // default: true
	LogSuspicious    *bool `yaml:"log_suspicious,omitempty"`     // default: true
	ConfidentialPrompt bool `yaml:"confidential_prompt,omitempty"` // default: false
}

// ShouldSanitize returns whether input sanitization is enabled (default: true).
func (c *SecurityConfig) ShouldSanitize() bool {
	if c == nil || c.SanitizeInput == nil {
		return true
	}
	return *c.SanitizeInput
}

// ShouldDetect returns whether injection detection is enabled (default: true).
func (c *SecurityConfig) ShouldDetect() bool {
	if c == nil || c.DetectInjection == nil {
		return true
	}
	return *c.DetectInjection
}

// ShouldFenceTools returns whether tool result fencing is enabled (default: true).
func (c *SecurityConfig) ShouldFenceTools() bool {
	if c == nil || c.FenceToolResults == nil {
		return true
	}
	return *c.FenceToolResults
}

// OTelConfig configures OpenTelemetry tracing export.
type OTelConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint,omitempty"` // OTLP endpoint (e.g., localhost:4317)
	Protocol string `yaml:"protocol,omitempty"` // grpc or http (default: grpc)
	Insecure bool   `yaml:"insecure,omitempty"` // skip TLS verification
}

// SkillsConfig defines skills attached to an agent.
type SkillsConfig struct {
	Attached []SkillRef `yaml:"attached,omitempty"`
}

// SkillRef is a reference to a skill directory.
type SkillRef struct {
	Path string `yaml:"path"`
}

// ExposeConfig allows an agent to expose itself as an MCP tool.
type ExposeConfig struct {
	AsTool      bool   `yaml:"as_tool"`
	ToolName    string `yaml:"tool_name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// OrchestratorConfig defines orchestration behavior for type: orchestrator agents.
type OrchestratorConfig struct {
	Agents  []OrchestratorAgent `yaml:"agents"`
	Handoff string              `yaml:"handoff,omitempty"` // full_output, summary, structured (default: full_output)
}

// OrchestratorAgent is a reference to a child agent in an orchestration DAG.
type OrchestratorAgent struct {
	Name      string          `yaml:"name"`
	Path      string          `yaml:"path"`
	DependsOn []string        `yaml:"depends_on,omitempty"`
	OutputSchema *HandoffSchema `yaml:"output_schema,omitempty"` // typed handoff validation
}

// HandoffSchema defines JSON Schema validation for inter-agent data transfer.
type HandoffSchema struct {
	Type       string                    `yaml:"type" json:"type"`
	Properties map[string]SchemaProperty `yaml:"properties,omitempty" json:"properties,omitempty"`
	Required   []string                  `yaml:"required,omitempty" json:"required,omitempty"`
	Strict     bool                      `yaml:"strict,omitempty" json:"-"` // error vs warning on mismatch
}

// SchemaProperty defines a single property in a handoff schema.
type SchemaProperty struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// IsOrchestrator returns true if this agent is an orchestrator type.
func (c *AgentConfig) IsOrchestrator() bool {
	return c.Type == "orchestrator" && c.Orchestrator != nil
}

// ModelConfig defines the model provider and parameters.
type ModelConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
	APIKeyEnv   string  `yaml:"api_key_env,omitempty"` // env var name for the API key
}

// ServerRef is a reference to an MCP server or an agent-as-tool.
type ServerRef struct {
	Name    string      `yaml:"name"`
	Path    string      `yaml:"path,omitempty"`
	Command string      `yaml:"command,omitempty"`
	URL     string      `yaml:"url,omitempty"`
	Agent   string      `yaml:"agent,omitempty"` // agent-as-tool path (e.g., ./agents/researcher)
	Tools   *ToolFilter `yaml:"tools,omitempty"`
}

// IsAgentRef returns true if this server ref points to an agent-as-tool.
func (s *ServerRef) IsAgentRef() bool {
	return s.Agent != ""
}

// ToolFilter controls which tools are exposed from a server.
type ToolFilter struct {
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// AgentSettings configures agent runtime behavior.
type AgentSettings struct {
	MaxToolCalls      int     `yaml:"max_tool_calls,omitempty"`
	TimeoutSecs       int     `yaml:"timeout_secs,omitempty"`
	Namespacing       string  `yaml:"namespacing,omitempty"`        // auto, always, never
	ParallelToolCalls bool    `yaml:"parallel_tool_calls,omitempty"` // execute concurrent tool calls in parallel
	BudgetPerRequest  float64 `yaml:"budget_per_request,omitempty"` // USD limit per request (0 = unlimited)
	BudgetMonthly     float64 `yaml:"budget_monthly,omitempty"`     // USD limit per calendar month (0 = unlimited)
}

// Load reads an agent.yaml from the given directory.
func Load(dir string) (*AgentConfig, error) {
	path := filepath.Join(dir, "agent.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent.yaml: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent.yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks the agent config for security and correctness issues.
func (c *AgentConfig) Validate() error {
	if c.Name != "" && !validAgentName.MatchString(c.Name) {
		return fmt.Errorf("invalid agent name %q: must be lowercase alphanumeric with hyphens, 1-63 chars, starting with a letter", c.Name)
	}
	return nil
}

// Save writes the agent config to agent.yaml in the given directory.
func Save(dir string, cfg *AgentConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal agent.yaml: %w", err)
	}

	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write agent.yaml: %w", err)
	}

	return nil
}
