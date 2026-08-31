// Package policy is Vigil's public judgment surface.
//
// Vigil is detective, not preventive (vigil:pdr-008): everything here reports
// and nothing here blocks. A check returns what it found; the caller decides
// what that means for the work in front of it. A host that chooses to stop on a
// violation is enforcing its own policy, not obeying Vigil — which is what
// keeps the guarantee Vigil makes about wrapped harnesses uniform, whether or
// not the harness also embeds this package.
//
// The types are Vigil's own rather than the session records it writes, so that
// a caller checking a string does not have to construct a session to do it.
package policy

import "fmt"

// Severity is how much a violation matters.
type Severity string

const (
	// Error is a violation a reasonable policy would stop for.
	Error Severity = "error"
	// Warning is worth recording and not worth stopping for.
	Warning Severity = "warning"
)

// Violation is one finding: what rule, how bad, and enough about where it was
// found for a person to go and look.
type Violation struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Source   string   `json:"source,omitempty"` // file path, tool name, or server
	Line     int      `json:"line,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
}

func (v Violation) String() string {
	where := v.Source
	if v.Line > 0 {
		where = fmt.Sprintf("%s:%d", v.Source, v.Line)
	}
	if where == "" {
		return fmt.Sprintf("[%s] %s", v.Severity, v.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", v.Severity, where, v.Message)
}

// Worst returns the highest severity present, or "" for none.
func Worst(vs []Violation) Severity {
	worst := Severity("")
	for _, v := range vs {
		if v.Severity == Error {
			return Error
		}
		worst = Warning
	}
	return worst
}
