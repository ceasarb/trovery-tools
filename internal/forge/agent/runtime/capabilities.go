package runtime

// Tuning capabilities per model (ADR-010).
//
// Separate from modelCosts because the two answer different questions and drift on
// different schedules: pricing changes when a model is repriced, capability changes when
// a model generation removes or adds a knob. They share a resolution rule — exact match
// first, then strip a dated snapshot suffix — so a dated ID and its alias always agree.
//
// Only *positive* knowledge lives here. An entry says "this model is known to reject
// this parameter"; the absence of an entry says nothing. See Capabilities for why the
// unknown case is permissive rather than restrictive.

// ModelCapabilities describes which tuning parameters a model accepts.
type ModelCapabilities struct {
	// Temperature reports whether the model accepts a non-default sampling
	// temperature. False on Claude Opus 4.7 and later, where temperature, top_p and
	// top_k were removed and sending one returns 400.
	Temperature bool

	// Effort reports whether the model supports output_config.effort.
	//
	// Not yet consumed — the `effort` field of ADR-010 §1 is not implemented. It is
	// populated now because the per-model rows are the expensive part of this table and
	// filling them twice is worse than carrying one field ahead of its consumer.
	Effort bool
}

// modelCapabilities holds models whose tuning surface differs from the permissive
// default. Keyed on the bare alias, never a dated snapshot — same rule as modelCosts.
var modelCapabilities = map[string]ModelCapabilities{
	// Claude 5 — temperature/top_p/top_k removed, full effort ladder.
	"claude-fable-5":  {Temperature: false, Effort: true},
	"claude-mythos-5": {Temperature: false, Effort: true},
	"claude-opus-5":   {Temperature: false, Effort: true},
	"claude-sonnet-5": {Temperature: false, Effort: true},

	// Claude 4.7 / 4.8 — first generation to remove sampling parameters.
	"claude-opus-4-8": {Temperature: false, Effort: true},
	"claude-opus-4-7": {Temperature: false, Effort: true},

	// Claude 4.6 — effort available, sampling parameters still accepted.
	"claude-opus-4-6":   {Temperature: true, Effort: true},
	"claude-sonnet-4-6": {Temperature: true, Effort: true},

	// Claude 4.5 — Opus supports effort; Sonnet and Haiku error on it.
	"claude-opus-4-5":   {Temperature: true, Effort: true},
	"claude-sonnet-4-5": {Temperature: true, Effort: false},
	"claude-haiku-4-5":  {Temperature: true, Effort: false},

	// Legacy Claude 4.0 — sampling only.
	"claude-sonnet-4-20250514": {Temperature: true, Effort: false},
	"claude-opus-4-20250514":   {Temperature: true, Effort: false},

	// OpenAI — deliberately permissive, and deliberately explicit.
	//
	// These rows say "considered, not characterised", which is different from an absent
	// row saying "never looked at". OpenAI's reasoning models are widely understood to
	// constrain sampling parameters, but ADR-010 §2 records the OpenAI wire shape as an
	// open question, and guessing here would refuse configurations that work today. So
	// they carry the permissive default until someone verifies against the live API —
	// at which point these rows are where the answer goes.
	//
	// Effort is false for the same reason: forge should not offer a knob whose
	// translation is unverified.
	"gpt-5.4":      {Temperature: true, Effort: false},
	"gpt-5.4-pro":  {Temperature: true, Effort: false},
	"gpt-5.2":      {Temperature: true, Effort: false},
	"gpt-5.1":      {Temperature: true, Effort: false},
	"gpt-5":        {Temperature: true, Effort: false},
	"gpt-5-mini":   {Temperature: true, Effort: false},
	"gpt-5-nano":   {Temperature: true, Effort: false},
	"gpt-4.1":      {Temperature: true, Effort: false},
	"gpt-4.1-mini": {Temperature: true, Effort: false},
	"gpt-4.1-nano": {Temperature: true, Effort: false},
	"gpt-4o":       {Temperature: true, Effort: false},
	"gpt-4o-mini":  {Temperature: true, Effort: false},

	// Ollama — local models take a sampling temperature and have no effort concept.
	"llama3.1":    {Temperature: true, Effort: false},
	"llama3.2":    {Temperature: true, Effort: false},
	"qwen2.5":     {Temperature: true, Effort: false},
	"mistral":     {Temperature: true, Effort: false},
	"deepseek-r1": {Temperature: true, Effort: false},
}

// defaultCapabilities is what an unlisted model gets: everything permitted.
//
// Permissive-on-unknown is deliberate, and it is the opposite of how modelCosts treats
// an unpriced model. The asymmetry is not an inconsistency — the two failure modes are
// not comparable:
//
//   - An unpriced model makes a configured budget *silently inert*. Nothing fails, the
//     service looks healthy, and the ceiling never fires. Only refusing to start
//     surfaces it, which is why requirePricedModel is a hard error.
//   - An unlisted model with an unsupported parameter fails *loudly at request time*
//     with a provider 400. Refusing to start adds nothing but the risk of blocking a
//     configuration that would have worked — a model released after this table was last
//     touched, or a local Ollama model with any name at all.
//
// So this table only ever converts a runtime 400 into a startup error for models we
// positively know about. It never invents a refusal from ignorance. The cost of a stale
// table is therefore the status quo (a 400 on first request), not a service that will
// not boot.
var defaultCapabilities = ModelCapabilities{Temperature: true, Effort: true}

// Capabilities resolves a model to its tuning capabilities.
//
// Resolution mirrors lookupCost: exact match, then strip a trailing -YYYYMMDD snapshot
// suffix and retry, so claude-opus-5 and a future dated form of it gate identically.
// Unlisted models get defaultCapabilities.
func Capabilities(model string) ModelCapabilities {
	if caps, ok := modelCapabilities[model]; ok {
		return caps
	}
	if base, ok := stripDateSuffix(model); ok {
		if caps, ok := modelCapabilities[base]; ok {
			return caps
		}
	}
	return defaultCapabilities
}

// SupportsTemperature reports whether the model accepts a non-default sampling
// temperature. False only for models this table positively knows reject it.
func SupportsTemperature(model string) bool {
	return Capabilities(model).Temperature
}
