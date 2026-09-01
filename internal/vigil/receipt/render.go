package receipt

import (
	"fmt"
	"strings"
)

// RenderText renders a receipt as the plain-text document printed to stdout.
//
// The receipt is the command's product, not console decoration, so it goes to
// stdout like SARIF does — pipeable and saveable.
func RenderText(r Receipt) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Agent receipt — Lumi run #%d\n", r.RunID)
	fmt.Fprintf(&b, "%s\n\n", HonestSentence)

	fmt.Fprintf(&b, "  When:        %s\n", r.At.Local().Format("2006-01-02 15:04:05"))
	// Request text is user-authored input, not an engine record — quoted and
	// attributed so it is never read as a statement of what happened.
	fmt.Fprintf(&b, "  Requested:   %q (as given by the user)\n", r.Request)
	fmt.Fprintf(&b, "  Attendance:  %s\n", r.Attendance)
	fmt.Fprintf(&b, "  Steps:       %d planned\n", r.Steps)
	fmt.Fprintf(&b, "  Outcome:     %s\n\n", r.Outcome())

	fmt.Fprintf(&b, "  Cost\n")
	fmt.Fprintf(&b, "    Forecast:  $%.4f\n", r.ForecastUSD)
	fmt.Fprintf(&b, "    Actual:    $%.4f (retries and escalations included)\n", r.ActualUSD)
	fmt.Fprintf(&b, "    Overhead:  $%.4f of actual (planning + classification)\n\n", r.OverheadUSD)

	fmt.Fprintf(&b, "  Policy:      %s\n", r.PolicyVersion)
	if r.UnderKit() {
		// What the run executed under, so "it answered from live data" is a
		// claim the reader can check rather than take.
		fmt.Fprintf(&b, "\n  Kit\n")
		fmt.Fprintf(&b, "    Name:      %s %s\n", r.Kit, r.KitVersion)
		if r.KitHash != "" {
			fmt.Fprintf(&b, "    Content:   %s\n", r.KitHash)
		}
		if r.Servers != "" {
			fmt.Fprintf(&b, "    Servers:   %s\n", r.Servers)
		}
		if r.Grants != "" {
			fmt.Fprintf(&b, "    Granted:   %s\n", r.Grants)
		}
	}
	if r.Flagged() {
		// Rendered as what it is: a heuristic objected, and the run carried on.
		// Stating it as a finding rather than a verdict is the whole posture —
		// the harness's own boundary is what stops anything.
		fmt.Fprintf(&b, "\n  Inspection flagged (reported, not blocked)\n")
		for _, line := range strings.Split(r.Flags, "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "  This receipt covers what the harness recorded for this run.\n")
	fmt.Fprintf(&b, "  Per-step detail — models used, effect-gate decisions — is not\n")
	fmt.Fprintf(&b, "  yet recorded by the harness and so cannot be certified here.\n")

	return b.String()
}
