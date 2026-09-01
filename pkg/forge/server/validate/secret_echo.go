package validate

import (
	"fmt"
	"strings"
)

// SecretEchoRuleID names the finding a response lint produces.
const SecretEchoRuleID = "security-echo-001"

// SecretEcho reports a tool response that contains the value of a credential
// the host injected into the server.
//
// Not a Rule, deliberately. Every other check in this package reads a tool
// *definition* — a Rule's Check takes one protocol.Tool and nothing else — and
// a definition cannot show this. What is being caught here happens later: a
// server is handed a credential to authenticate with, and hands it back inside
// an answer, where it reaches a model, a log, a receipt, and whoever reads any
// of them.
//
// The host is the only party that can run this check, because the host is the
// only party that knows the values. That is exactly the arrangement the
// credential rule (Lumi ADR-012) sets up — secrets are user-supplied and
// host-held, never kit content — and it is what makes the check possible
// rather than a guess at what a secret looks like. No pattern matching, no
// entropy heuristic: either the response contains the string that was injected
// or it does not.
//
// secrets maps a credential's name to its value. Empty and whitespace-only
// values are skipped: a secret that was never supplied has no value to echo,
// and matching on "" would report every response ever written.
func SecretEcho(toolName, response string, secrets map[string]string) []Violation {
	if strings.TrimSpace(response) == "" || len(secrets) == 0 {
		return nil
	}
	var out []Violation
	for name, value := range secrets {
		if len(strings.TrimSpace(value)) < minSecretLen {
			continue
		}
		if !strings.Contains(response, value) {
			continue
		}
		out = append(out, Violation{
			RuleID:   SecretEchoRuleID,
			Category: CategorySecurity,
			Severity: SeverityError,
			ToolName: toolName,
			Message: fmt.Sprintf(
				"response contains the value of %s, a credential the host injected into this server", name),
			Suggestion: "A server authenticates with a credential; it never returns one. " +
				"Remove the value from the response — echoing it puts it in front of the model, " +
				"the transcript and the receipt.",
		})
	}
	return out
}

// minSecretLen is the shortest value worth matching on.
//
// A three-character credential would appear inside ordinary prose constantly,
// and every one of those reports would be false. Short values are skipped
// rather than reported, and this is recorded as a real limit of the check: a
// pathologically short secret is not covered by it.
const minSecretLen = 8
