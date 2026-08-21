package runtime

import "testing"

func TestSupportsTemperature(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		// The models the guard exists for: temperature removed on Opus 4.7+.
		{"opus 5", "claude-opus-5", false},
		{"opus 4.8", "claude-opus-4-8", false},
		{"opus 4.7", "claude-opus-4-7", false},
		{"sonnet 5", "claude-sonnet-5", false},
		{"fable 5", "claude-fable-5", false},
		{"mythos 5", "claude-mythos-5", false},

		// Still accepted — including the stock template model, which is why the
		// default quickstart path is unaffected by the guard.
		{"haiku 4.5", "claude-haiku-4-5", true},
		{"opus 4.6", "claude-opus-4-6", true},
		{"sonnet 4.6", "claude-sonnet-4-6", true},
		{"sonnet 4.5", "claude-sonnet-4-5", true},

		// Local models take a sampling temperature.
		{"ollama llama3.1", "llama3.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsTemperature(tt.model); got != tt.want {
				t.Errorf("SupportsTemperature(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// A dated snapshot and its alias name the same model, so they must gate identically.
// This mirrors lookupCost's resolution and is the bug class ADR-009 was written about,
// reproduced on the capability axis.
func TestSupportsTemperatureNormalizesDatedSnapshot(t *testing.T) {
	if !SupportsTemperature("claude-haiku-4-5-20251001") {
		t.Error("dated Haiku snapshot should resolve to the alias and permit temperature")
	}
	if SupportsTemperature("claude-opus-5-20260115") {
		t.Error("dated Opus 5 snapshot should resolve to the alias and reject temperature")
	}
}

// An unlisted model must stay permissive. Refusing here would convert a model newer
// than this binary — or any locally-named Ollama model — from "works" into "will not
// boot", which is strictly worse than the provider's own 400.
func TestUnknownModelIsPermissive(t *testing.T) {
	for _, model := range []string{
		"claude-opus-9",         // released after this binary
		"my-finetune:latest",    // local Ollama tag
		"",                      // unset in config
		"claude-opus-5-turbo-x", // suffix that is not a date
	} {
		if !SupportsTemperature(model) {
			t.Errorf("SupportsTemperature(%q) = false, want true (unknown must not be refused)", model)
		}
	}
}

// A non-date suffix must not be mistaken for a snapshot and stripped back to a
// restrictive alias.
func TestNonDateSuffixIsNotStripped(t *testing.T) {
	if !SupportsTemperature("claude-opus-5-alpha0001") {
		t.Error("non-date suffix should not resolve to the claude-opus-5 entry")
	}
}

func TestCapabilitiesEffort(t *testing.T) {
	// Effort is not consumed yet, but the rows are load-bearing for ADR-010 §1 and the
	// two exceptions are easy to get wrong: Sonnet/Haiku 4.5 error on effort while
	// Opus 4.5 accepts it.
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-opus-5", true},
		{"claude-sonnet-4-6", true},
		{"claude-opus-4-5", true},
		{"claude-sonnet-4-5", false},
		{"claude-haiku-4-5", false},
		{"llama3.1", false},
	}

	for _, tt := range tests {
		if got := Capabilities(tt.model).Effort; got != tt.want {
			t.Errorf("Capabilities(%q).Effort = %v, want %v", tt.model, got, tt.want)
		}
	}
}

// Every model forge prices should have a capability row. A model that is priced but
// unlisted here silently falls back to permissive, which is the drift this pairing is
// meant to make visible.
func TestPricedModelsHaveCapabilityRows(t *testing.T) {
	for model := range modelCosts {
		if _, ok := modelCapabilities[model]; !ok {
			t.Errorf("model %q is priced in modelCosts but has no modelCapabilities row", model)
		}
	}
}
