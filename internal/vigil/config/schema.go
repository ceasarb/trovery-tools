package config

// WebhookConfig defines a webhook endpoint.
type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Events  []string          `yaml:"events,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Format  string            `yaml:"format,omitempty"` // "json" (default) or "slack"
}

// VigilConfig is the top-level configuration parsed from .demi/vigil.yaml.
type VigilConfig struct {
	Name     string                `yaml:"name"`
	Version  string                `yaml:"version"`
	Tools    map[string]ToolConfig `yaml:"tools,omitempty"`
	Policies Policies              `yaml:"policies"`
	Skills   SkillsConfig          `yaml:"skills,omitempty"`
	Tracking TrackingConfig        `yaml:"tracking"`
	Webhooks []WebhookConfig       `yaml:"webhooks,omitempty"`
}

// ToolConfig configures a single AI tool adapter.
type ToolConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Reason    string            `yaml:"reason,omitempty"`
	Config    map[string]string `yaml:"config,omitempty"`
	AgentPath string            `yaml:"agent_path,omitempty"`
}

// Policies groups all policy configurations.
type Policies struct {
	Secrets    SecretsPolicy    `yaml:"secrets"`
	Filesystem FilesystemPolicy `yaml:"filesystem"`
}

// SecretsPolicy defines secret detection rules.
type SecretsPolicy struct {
	ScanAgentOutput    bool     `yaml:"scan_agent_output"`
	ScanCommits        bool     `yaml:"scan_commits"`
	BlockPatterns      []string `yaml:"block_patterns"`
	CustomPatternsFile string   `yaml:"custom_patterns_file,omitempty"`
}

// FilesystemPolicy defines filesystem access rules.
type FilesystemPolicy struct {
	ReadOnly     []string `yaml:"read_only,omitempty"`
	NoWrite      []string `yaml:"no_write,omitempty"`
	AllowedWrite []string `yaml:"allowed_write,omitempty"`
}

// SkillsConfig defines skill governance settings.
type SkillsConfig struct {
	ScanPaths []string                `yaml:"scan_paths,omitempty"`
	Global    SkillsGlobalPolicy      `yaml:"global,omitempty"`
	Contexts  map[string]SkillsPolicy `yaml:"contexts,omitempty"`
}

// SkillsGlobalPolicy is the global skill governance policy.
type SkillsGlobalPolicy struct {
	Policy  string   `yaml:"policy"`
	Allowed []string `yaml:"allowed,omitempty"`
	Blocked []string `yaml:"blocked,omitempty"`
}

// SkillsPolicy is a context-specific skill governance policy.
type SkillsPolicy struct {
	Policy  string   `yaml:"policy"`
	Allowed []string `yaml:"allowed,omitempty"`
	Blocked []string `yaml:"blocked,omitempty"`
}

// TrackingConfig controls session tracking behavior.
type TrackingConfig struct {
	LogFileChanges bool   `yaml:"log_file_changes"`
	LogCommands    bool   `yaml:"log_commands"`
	TrackTokens    bool   `yaml:"track_tokens"`
	TrackCost      bool   `yaml:"track_cost"`
	SessionDir     string `yaml:"session_dir"`
	ExportFormat   string `yaml:"export_format"`
}

// DefaultConfig returns a VigilConfig with sensible defaults.
func DefaultConfig() VigilConfig {
	return VigilConfig{
		Version: "0.1.0",
		Policies: Policies{
			Secrets: SecretsPolicy{
				ScanAgentOutput: true,
				ScanCommits:     true,
				BlockPatterns: []string{
					`AWS_SECRET_ACCESS_KEY\s*=\s*\S+`,
					`PRIVATE_KEY`,
					`password\s*=\s*\S+`,
					`Bearer\s+[A-Za-z0-9\-._~+/]{20,}`,
					`ghp_[A-Za-z0-9]{36}`,
					`sk-[A-Za-z0-9]{32,}`,
				},
			},
			Filesystem: FilesystemPolicy{
				ReadOnly: []string{".env*", "secrets/", ".git/", "*.pem", "*.key"},
			},
		},
		Tracking: TrackingConfig{
			LogFileChanges: true,
			LogCommands:    true,
			TrackTokens:    true,
			TrackCost:      true,
			SessionDir:     ".demi/vigil/sessions/",
			ExportFormat:   "json",
		},
	}
}
