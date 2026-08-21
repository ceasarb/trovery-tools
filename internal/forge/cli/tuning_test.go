package cli

import (
	"strings"
	"testing"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/eval"
)

func TestRequireSupportedTuningRejectsTemperatureOnRestrictiveModel(t *testing.T) {
	err := requireSupportedTuning(&agentcfg.ModelConfig{
		Provider:    "anthropic",
		Model:       "claude-opus-5",
		Temperature: 0.7,
	})
	if err == nil {
		t.Fatal("expected refusal for temperature on claude-opus-5")
	}
	// The message has to carry both the model and the offending value, or the operator
	// has to go hunting for which line to delete.
	if !strings.Contains(err.Error(), "claude-opus-5") || !strings.Contains(err.Error(), "0.7") {
		t.Errorf("error should name the model and the value, got: %v", err)
	}
}

// The stock templates ship temperature over Haiku 4.5. If this ever fails, the
// quickstart is broken by the guard meant to protect it.
func TestRequireSupportedTuningAllowsStockTemplateShape(t *testing.T) {
	for _, model := range []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001"} {
		err := requireSupportedTuning(&agentcfg.ModelConfig{
			Provider:    "anthropic",
			Model:       model,
			Temperature: 0.7,
		})
		if err != nil {
			t.Errorf("model %q: unexpected refusal: %v", model, err)
		}
	}
}

// Temperature is only refused when actually set. An agent on a restrictive model that
// never configured one must start normally — this is the common case and the guard
// must be invisible to it.
func TestRequireSupportedTuningIgnoresUnsetTemperature(t *testing.T) {
	if err := requireSupportedTuning(&agentcfg.ModelConfig{
		Provider: "anthropic",
		Model:    "claude-opus-5",
	}); err != nil {
		t.Errorf("unset temperature must not be refused: %v", err)
	}
}

func TestRequireSupportedTuningAllowsUnknownModel(t *testing.T) {
	if err := requireSupportedTuning(&agentcfg.ModelConfig{
		Provider:    "ollama",
		Model:       "my-local-finetune:latest",
		Temperature: 0.9,
	}); err != nil {
		t.Errorf("unknown model must not be refused: %v", err)
	}
}

// A suite-level settings.model override is the model that actually runs, so it is what
// the guard has to check — an agent config naming a permissive model does not make a
// restrictive suite override safe.
func TestRequireSupportedTuningForSuitesHonoursSuiteModelOverride(t *testing.T) {
	cfg := &agentcfg.AgentConfig{
		Model: agentcfg.ModelConfig{
			Provider:    "anthropic",
			Model:       "claude-haiku-4-5",
			Temperature: 0.3,
		},
	}
	suites := []*eval.Suite{
		{Name: "correctness", Settings: eval.Settings{Model: "claude-opus-5"}},
	}

	err := requireSupportedTuningForSuites(cfg, suites)
	if err == nil {
		t.Fatal("expected refusal: suite overrides onto a model that rejects temperature")
	}
	if !strings.Contains(err.Error(), "correctness") {
		t.Errorf("error should name the suite, got: %v", err)
	}
}

func TestRequireSupportedTuningForSuitesUsesAgentModelWhenNoOverride(t *testing.T) {
	cfg := &agentcfg.AgentConfig{
		Model: agentcfg.ModelConfig{
			Provider:    "anthropic",
			Model:       "claude-haiku-4-5",
			Temperature: 0.3,
		},
	}
	suites := []*eval.Suite{{Name: "correctness"}}

	if err := requireSupportedTuningForSuites(cfg, suites); err != nil {
		t.Errorf("unexpected refusal: %v", err)
	}
}

// Unlike the cost guard, this one is not scoped to suites that opt into anything: a
// rejected parameter 400s every request, so a pure correctness suite breaks too.
func TestRequireSupportedTuningForSuitesAppliesWithoutCostAssertions(t *testing.T) {
	cfg := &agentcfg.AgentConfig{
		Model: agentcfg.ModelConfig{
			Provider:    "anthropic",
			Model:       "claude-opus-5",
			Temperature: 0.7,
		},
	}
	suites := []*eval.Suite{{Name: "no-cost-assertions"}}

	if err := requireSupportedTuningForSuites(cfg, suites); err == nil {
		t.Error("expected refusal even though the suite asserts nothing about cost")
	}
}
