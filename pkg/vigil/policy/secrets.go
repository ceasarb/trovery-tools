package policy

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// DefaultSecretPatterns is what Vigil looks for when a caller states no
// patterns of its own.
//
// The same list the shipped config templates carry, kept here so a caller with
// no Vigil project — packaging a kit, scanning a string — gets the useful
// behaviour without constructing a config file to hold a copy of it.
var DefaultSecretPatterns = []string{
	`AWS_SECRET_ACCESS_KEY\s*=\s*\S+`,
	`PRIVATE_KEY`,
	`password\s*=\s*\S+`,
	`Bearer\s+[A-Za-z0-9\-._~+/]{20,}`,
	`ghp_[A-Za-z0-9]{36}`,
	`sk-[A-Za-z0-9]{32,}`,
	`npm_[A-Za-z0-9]{36}`,
	`xox[baprs]-[A-Za-z0-9-]{10,}`,
	`AKIA[0-9A-Z]{16}`,
}

// Secrets finds secret-shaped content in text.
type Secrets struct {
	patterns []*regexp.Regexp
	names    []string
}

// NewSecrets compiles a scanner. Empty patterns means DefaultSecretPatterns.
//
// An unparseable pattern is an error rather than a warning: a scanner that
// silently drops a rule reports "clean" for content the caller believed was
// being checked, which is worse than refusing to start.
func NewSecrets(patterns ...string) (*Secrets, error) {
	if len(patterns) == 0 {
		patterns = DefaultSecretPatterns
	}
	s := &Secrets{}
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("policy: secret pattern %q: %w", p, err)
		}
		s.patterns = append(s.patterns, re)
		s.names = append(s.names, p)
	}
	return s, nil
}

// PatternCount reports how many rules are loaded.
func (s *Secrets) PatternCount() int { return len(s.patterns) }

// ScanContent reports every line of content matching a secret pattern.
//
// source names where the content came from — a file path, a tool name — and is
// echoed back in the violation so a caller scanning many things can tell them
// apart. The matched text is deliberately not included: a report that quotes
// the secret it found spreads it into logs, and the line number is enough to
// go and look.
func (s *Secrets) ScanContent(content, source string) []Violation {
	var out []Violation
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		for i, re := range s.patterns {
			if re.MatchString(text) {
				out = append(out, Violation{
					Rule:     "secrets.block_patterns",
					Severity: Error,
					Message:  fmt.Sprintf("possible secret: pattern %q matched", s.names[i]),
					Source:   source,
					Line:     line,
					Pattern:  s.names[i],
				})
			}
		}
	}
	return out
}
