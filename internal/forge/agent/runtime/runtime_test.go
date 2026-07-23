package runtime

import (
	"testing"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

func TestEstimatedCostHaiku(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "claude-haiku-4-5-20251001"},
		},
		TotalInput:  1_000_000,
		TotalOutput: 1_000_000,
	}

	cost := session.EstimatedCost()
	// Haiku: $1/MTok input + $5/MTok output = $6
	if cost != 6.0 {
		t.Errorf("cost = %f, want 6.0", cost)
	}
}

func TestEstimatedCostSonnet(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "claude-sonnet-4-6"},
		},
		TotalInput:  500_000,
		TotalOutput: 100_000,
	}

	cost := session.EstimatedCost()
	// Sonnet: $3*0.5 + $15*0.1 = $1.5 + $1.5 = $3.0
	if cost != 3.0 {
		t.Errorf("cost = %f, want 3.0", cost)
	}
}

func TestEstimatedCostOpenAI(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "gpt-5-mini"},
		},
		TotalInput:  1_000_000,
		TotalOutput: 1_000_000,
	}

	cost := session.EstimatedCost()
	// gpt-5-mini: $0.25 + $2.00 = $2.25
	if cost != 2.25 {
		t.Errorf("cost = %f, want 2.25", cost)
	}
}

func TestEstimatedCostUnknownModel(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "unknown-model"},
		},
		TotalInput:  1_000_000,
		TotalOutput: 1_000_000,
	}

	cost := session.EstimatedCost()
	if cost != 0 {
		t.Errorf("cost = %f, want 0 for unknown model", cost)
	}
}

func TestEstimatedCostZeroTokens(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "claude-haiku-4-5-20251001"},
		},
	}

	cost := session.EstimatedCost()
	if cost != 0 {
		t.Errorf("cost = %f, want 0", cost)
	}
}

func TestSummary(t *testing.T) {
	session := &Session{
		Config: &agentcfg.AgentConfig{
			Model: agentcfg.ModelConfig{Model: "claude-haiku-4-5-20251001"},
		},
		TotalInput:  100,
		TotalOutput: 50,
		ToolCalls:   3,
	}

	summary := session.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestModelCostsComplete(t *testing.T) {
	// Verify all key models are in the cost table
	expected := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-haiku-4-5-20251001",
		"gpt-5.4",
		"gpt-5-mini",
		"gpt-5-nano",
		"gpt-4.1",
		"gpt-4o",
	}

	for _, model := range expected {
		if _, ok := modelCosts[model]; !ok {
			t.Errorf("model %q missing from cost table", model)
		}
	}
}
