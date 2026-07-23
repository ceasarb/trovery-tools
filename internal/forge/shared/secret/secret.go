package secret

import "fmt"

// Str wraps a sensitive string value and redacts it in all string representations.
type Str struct {
	val string
}

// New creates a new SecretStr wrapping the given value.
func New(val string) Str {
	return Str{val: val}
}

// Value returns the actual secret value. Use sparingly — only when
// the value must be passed to an external API (e.g., HTTP Authorization header).
func (s Str) Value() string {
	return s.val
}

// String returns a redacted representation.
func (s Str) String() string {
	return "***"
}

// GoString returns a redacted Go syntax representation.
func (s Str) GoString() string {
	return "secret.Str{***}"
}

// Format implements fmt.Formatter to redact in all format verbs.
func (s Str) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') || f.Flag('#') {
			fmt.Fprint(f, "secret.Str{***}")
		} else {
			fmt.Fprint(f, "***")
		}
	default:
		fmt.Fprint(f, "***")
	}
}

// MarshalJSON returns a JSON-safe redacted string.
func (s Str) MarshalJSON() ([]byte, error) {
	return []byte(`"***"`), nil
}

// MarshalYAML returns a YAML-safe redacted string.
func (s Str) MarshalYAML() (interface{}, error) {
	return "***", nil
}

// IsEmpty returns true if the secret has no value.
func (s Str) IsEmpty() bool {
	return s.val == ""
}
