package config

// Template represents a predefined agent configuration.
type Template struct {
	Name        string
	Description string
	Config      AgentConfig
}

// Templates returns all available agent templates.
func Templates() []Template {
	return []Template{
		{
			Name:        "single-agent",
			Description: "General-purpose agent with a single system prompt",
			Config: AgentConfig{
				Model: ModelConfig{
					Provider:    "anthropic",
					APIKeyEnv:   "ANTHROPIC_API_KEY",
					Model:       "claude-haiku-4-5-20251001",
					MaxTokens:   8192,
					Temperature: 0.7,
				},
				System: "You are a helpful assistant. Use the available tools to help the user accomplish their tasks.",
				Settings: AgentSettings{
					MaxToolCalls: 25,
					TimeoutSecs:  120,
					Namespacing:  "auto",
				},
			},
		},
		{
			Name:        "researcher",
			Description: "Research-focused agent optimized for information gathering",
			Config: AgentConfig{
				Model: ModelConfig{
					Provider:    "anthropic",
					APIKeyEnv:   "ANTHROPIC_API_KEY",
					Model:       "claude-haiku-4-5-20251001",
					MaxTokens:   8192,
					Temperature: 0.3,
				},
				System: "You are a research assistant. Gather information thoroughly using available tools. Cite your sources and distinguish between facts and inferences.",
				Settings: AgentSettings{
					MaxToolCalls: 50,
					TimeoutSecs:  180,
					Namespacing:  "auto",
				},
			},
		},
		{
			Name:        "custom",
			Description: "Minimal agent config — configure everything yourself",
			Config: AgentConfig{
				Model: ModelConfig{
					Provider:  "anthropic",
					APIKeyEnv: "ANTHROPIC_API_KEY",
					Model:     "claude-haiku-4-5-20251001",
					MaxTokens: 4096,
				},
				System: "You are a helpful assistant.",
				Settings: AgentSettings{
					MaxToolCalls: 10,
					TimeoutSecs:  60,
					Namespacing:  "auto",
				},
			},
		},
	}
}
