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

	fmt.Fprintf(&b, "  Policy:      %s\n\n", r.PolicyVersion)

	fmt.Fprintf(&b, "  This receipt covers what the harness recorded for this run.\n")
	fmt.Fprintf(&b, "  Per-step detail — models used, effect-gate decisions — is not\n")
	fmt.Fprintf(&b, "  yet recorded by the harness and so cannot be certified here.\n")

	return b.String()
}
