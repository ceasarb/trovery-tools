package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/security"
)

// ToolResult is one tool's answer, offered for inspection before a host acts
// on it.
type ToolResult struct {
	// Tool and Server name where the content came from, for the report.
	Tool   string
	Server string

	// Content is what the tool returned.
	Content string

	// Evidence is everything the host already trusted before this result
	// arrived — the user's request, the task it compiled to, material carried
	// from earlier steps. Used to tell content the caller supplied from
	// content that arrived from outside.
	Evidence string
}

// CheckToolResult inspects a tool's output and reports what it finds.
//
// This is the in-flight surface vigil:pdr-008 does not have and deliberately
// will not grow: it blocks nothing and is wired into nothing. A host calls it
// when it wants an opinion, and remains the only thing that can stop work. The
// uniform guarantee pdr-008 protects is about what `vigil run` does to a
// wrapped harness, which this does not change.
//
// Findings are Warning, not Error. Detection of injected instructions is a
// heuristic over adversarial input, and a heuristic that reports certainty is
// lying; the host's own boundary is what makes the guarantee. Callers that stop
// on warnings from tool output are welcome to — that is their policy.
func CheckToolResult(r ToolResult) []Violation {
	var out []Violation
	source := r.Tool
	if r.Server != "" {
		source = r.Server + "/" + r.Tool
	}

	for i, line := range strings.Split(r.Content, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 12 {
			continue
		}
		// Content the caller already had is not an intrusion. A request that
		// says "ignore the earlier instructions" is the user talking.
		if r.Evidence != "" && strings.Contains(strings.ToLower(r.Evidence), strings.ToLower(line)) {
			continue
		}
		if rule, ok := instructionShaped(line); ok {
			out = append(out, Violation{
				Rule:     rule,
				Severity: Warning,
				Message: fmt.Sprintf("tool output reads as an instruction rather than data: %q",
					truncate(line, 160)),
				Source: source,
				Line:   i + 1,
			})
		}
	}
	return out
}

// directive matches text telling an assistant what to do.
//
// Vigil's own rather than security.DetectInjection alone, because that
// detector misses common phrasings — notably the possessive, "ignore *your*
// previous instructions" — and a check that inherits one detector's blind spots
// hands them to every caller. Both run; either one is enough to report.
var directive = regexp.MustCompile(
	`(?i)\b(ignore|disregard|forget|override)\b[^.]{0,40}\b(instruction|instructions|rules|prompt|above|previous|prior)\b` +
		`|(?i)\b(you are now|from now on|new instructions?|system:|assistant:)\b` +
		`|(?i)\b(transfer|send|wire|delete|email|publish|post)\b[^.]{0,60}\b(fund|funds|money|payment|everyone|all users|account)\b`)

func instructionShaped(line string) (rule string, ok bool) {
	if security.DetectInjection(line).IsSuspicious {
		return "toolresult.injection", true
	}
	if directive.MatchString(line) {
		return "toolresult.directive", true
	}
	return "", false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
