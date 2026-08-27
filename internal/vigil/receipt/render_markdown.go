package receipt

import (
	"fmt"
	"strings"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/security"
)

// RenderMarkdown renders a receipt as a shareable Markdown document.
//
// Untrusted text (today: the user-authored request) is sanitized and placed
// inside a code fence sized longer than any backtick run it contains, so text
// carrying Markdown syntax — headings, links, fence markers — renders as
// verbatim data and can never restructure the receipt around it. Same trust
// posture as the runtime's tool-result fencing, applied to the document.
func RenderMarkdown(r Receipt) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Agent receipt — Lumi run #%d\n\n", r.RunID)
	fmt.Fprintf(&b, "**%s**\n\n", HonestSentence)

	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| When | %s |\n", r.At.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "| Attendance | %s |\n", r.Attendance)
	fmt.Fprintf(&b, "| Steps | %d planned |\n", r.Steps)
	fmt.Fprintf(&b, "| Outcome | %s |\n", r.Outcome())
	fmt.Fprintf(&b, "| Policy | %s |\n\n", r.PolicyVersion)

	fmt.Fprintf(&b, "**Requested** — as given by the user, shown verbatim as data:\n\n")
	fmt.Fprintf(&b, "%s\n\n", fenceUntrusted(r.Request))

	fmt.Fprintf(&b, "## Cost\n\n")
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Forecast | $%.4f |\n", r.ForecastUSD)
	fmt.Fprintf(&b, "| Actual | $%.4f (retries and escalations included) |\n", r.ActualUSD)
	fmt.Fprintf(&b, "| Overhead | $%.4f of actual (planning + classification) |\n\n", r.OverheadUSD)

	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "Source: `%s` (run %d).\n\n", r.Source, r.RunID)
	fmt.Fprintf(&b, "*This receipt covers what the harness recorded for this run. "+
		"Per-step detail — models used, effect-gate decisions — is not yet recorded "+
		"by the harness and so cannot be certified here.*\n")

	return b.String()
}

// fenceUntrusted sanitizes text and wraps it in a code fence longer than any
// backtick run inside it, so the content cannot close the fence early.
func fenceUntrusted(s string) string {
	s = security.SanitizeInput(s)

	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	f := strings.Repeat("`", n)
	return f + "\n" + s + "\n" + f
}
