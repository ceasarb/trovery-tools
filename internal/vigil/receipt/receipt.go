// Package receipt turns a recorded agent run into a document a person reads.
//
// This is the witness half of the personal-agent trust layer (PDR-010): Vigil
// reads what a harness recorded about a run and renders it as a receipt. Per
// ADR-012 the evidence comes from the harness's own event records — the
// harness engine is the trusted computing base, the model is not — so the
// receipt's claim is exactly "recorded by the harness, outside the model's
// reach", and never more than that.
//
// Anything model- or user-authored that appears in a receipt (today: the
// request text) is rendered quoted and attributed, never stated as fact. What
// happened comes only from engine-written records.
//
// Gaps — fields a receipt wants that Lumi's harness does not yet record.
// ADR-012 sends these fixes to Lumi's engine (adding to what the TCB
// records), not here:
//
//  0. Whole runs can go unrecorded: Engine.Execute's trivial fast path
//     (`if p.Trivial`) and its step-failure exits (runStep error other than
//     held; done-when not satisfied) all return without e.record(), so a
//     completed trivial answer or a failed run leaves no run row at all —
//     invisible to any receipt. Verified live 2026-08-27 on both paths.
//  1. Per-step records: step descriptions, model/provider per step, per-step
//     cost, attempts and failovers. contract.Ledger holds these in memory and
//     they die at process exit; only the step count survives.
//  2. Per-effect gate decisions: which steps crossed the effect boundary and
//     what was approved or held. Only the run-level held flag survives.
//  3. No join key between the runs table (autoincrement id) and the waiting
//     table (Prepared id string), so a held run's reason cannot be attached
//     to its receipt without guessing.
//  4. Models used are not recorded on the run, so eval-floor evidence
//     (store.EvalResult) cannot be cited by a receipt.
//  5. The run id is never surfaced to the user at run time, so "receipt for
//     the run you just did" requires listing.
package receipt

import "time"

// HonestSentence is the exact trust claim a receipt is allowed to make
// (ADR-012). Wording stronger than this — anything implying the witness is
// independent of the harness — overclaims and must not appear.
const HonestSentence = "Recorded by the harness, outside the model's reach."

// Receipt is one run, as the harness recorded it.
type Receipt struct {
	Source string // path of the store this receipt was read from
	RunID  int64

	At         time.Time
	Request    string // user-authored; render quoted, never as fact
	Steps      int    // planned step count (per-step detail: see gap 1)
	Attendance string // attended | unattended

	ContractMet bool
	Held        bool // stopped at a hold for a person (ADR-004 in Lumi)

	ForecastUSD   float64
	ActualUSD     float64 // effective cost: retries and escalations included
	OverheadUSD   float64 // planning + classification share of actual
	PolicyVersion string

	// Kit provenance: what the run executed under, when a kit was involved.
	//
	// Empty for a run that used no kit, and empty for a store written by a
	// Lumi that predates these columns — the witness reports what the record
	// holds and never fills a gap in with a guess.
	Kit        string // installed kit name
	KitVersion string // version from its manifest
	KitHash    string // content hash approved at install
	Servers    string // servers the run attached, comma-separated
	Grants     string // capabilities the kit was approved for
}

// UnderKit reports whether this run has kit provenance recorded.
func (r Receipt) UnderKit() bool { return r.Kit != "" }

// Outcome is the one-line verdict the engine's flags support.
func (r Receipt) Outcome() string {
	switch {
	case r.Held:
		return "stopped — held for a person"
	case r.ContractMet:
		return "contract met"
	default:
		return "ended without meeting the contract"
	}
}
