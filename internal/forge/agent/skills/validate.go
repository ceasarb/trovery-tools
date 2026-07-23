package skills

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationResult holds the outcome of validating a SKILL.md.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Validate checks a skill for correctness and best practices.
func Validate(skill *Skill) *ValidationResult {
	r := &ValidationResult{Valid: true}

	// Required: name
	if skill.Name == "" {
		r.addError("missing required field: name")
	} else {
		if len(skill.Name) > 64 {
			r.addError("name must be 64 characters or fewer")
		}
		if !namePattern.MatchString(skill.Name) {
			r.addError("name must be lowercase alphanumeric with hyphens (e.g., code-review)")
		}
	}

	// Required: description
	if skill.Description == "" {
		r.addError("missing required field: description")
	} else if len(skill.Description) > 1024 {
		r.addError("description must be 1024 characters or fewer")
	}

	// No XML tags in name or description
	if containsXMLTags(skill.Name) {
		r.addError("name must not contain XML tags")
	}
	if containsXMLTags(skill.Description) {
		r.addError("description must not contain XML tags")
	}

	// Namespace validation (optional but if present, must be valid)
	if skill.Namespace != "" && !namePattern.MatchString(skill.Namespace) {
		r.addError("namespace must be lowercase alphanumeric with hyphens")
	}

	// Body should not be empty
	if strings.TrimSpace(skill.Body) == "" {
		r.addWarning("SKILL.md body is empty — skill has no instructions")
	}

	// Version format (optional)
	if skill.Version != "" {
		if !isValidSemver(skill.Version) {
			r.addWarning("version should follow semver format (e.g., 0.1.0)")
		}
	}

	return r
}

// ValidateDirectory validates a skill directory at the given path.
func ValidateDirectory(dir string) (*Skill, *ValidationResult) {
	skill, err := ParseSkill(dir)
	if err != nil {
		return nil, &ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}
	}
	return skill, Validate(skill)
}

func (r *ValidationResult) addError(msg string) {
	r.Valid = false
	r.Errors = append(r.Errors, msg)
}

func (r *ValidationResult) addWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

func containsXMLTags(s string) bool {
	return strings.Contains(s, "<") && strings.Contains(s, ">")
}

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func isValidSemver(v string) bool {
	return semverPattern.MatchString(v)
}

// FormatResult returns a human-readable string of validation results.
func FormatResult(r *ValidationResult) string {
	var parts []string
	for _, e := range r.Errors {
		parts = append(parts, fmt.Sprintf("  ✗ %s", e))
	}
	for _, w := range r.Warnings {
		parts = append(parts, fmt.Sprintf("  ⚠ %s", w))
	}
	return strings.Join(parts, "\n")
}
