// Package skill is the single SKILL.md parser shared by both binaries.
//
// It replaces the two previously-duplicated parsers — Forge's
// internal/agent/skills/parser.go and Vigil's internal/skills/parser.go plus
// schema.go — whose independent drift is what produced the `requires` type
// conflict (see ADR-003, ADR-006). One implementation means that class of
// drift becomes a compile error instead of a review someone has to remember.
package skill

// Metadata keys for this platform's harness config, namespaced under a
// dotted `trove.*` prefix so they don't collide with other clients writing
// into the spec's shared metadata map (ADR-003).
const (
	MetaNamespace = "trove.namespace"
	MetaVersion   = "trove.version"
)

// Legacy keys read as a fallback for skills authored before a rebrand, newest
// prefix first: demi.* (the Demigo era), then tandem.* before that. We emit
// trove.* but still read both, so older SKILL.md files keep resolving.
var (
	legacyMetaNamespace = []string{"demi.namespace", "tandem.namespace"}
	legacyMetaVersion   = []string{"demi.version", "tandem.version"}
)

// Skill is the parsed representation of a SKILL.md file: its frontmatter
// metadata plus the markdown body.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Tags        []string `yaml:"tags"`

	// Metadata is the spec's optional string→string map. Harness config rides
	// here under trove.* keys (ADR-003); Namespace and Version are resolved
	// from it after parsing.
	Metadata map[string]string `yaml:"metadata"`

	// Namespace and Version are resolved from Metadata's trove.* keys (or the
	// legacy tandem.* keys), falling back to legacy top-level
	// `namespace:`/`version:` frontmatter. Not unmarshalled directly — see resolveMetadata.
	Namespace string `yaml:"-"`
	Version   string `yaml:"-"`

	Body string `yaml:"-"` // markdown below the frontmatter
	Path string `yaml:"-"` // directory (ParseDir) or file (ParseFile)
}

// Identity returns the namespace/name identifier, defaulting the namespace to
// "local" when unset. This is the identifier Forge uses to register and display
// skills.
func (s *Skill) Identity() string {
	ns := s.Namespace
	if ns == "" {
		ns = "local"
	}
	return ns + "/" + s.Name
}

// FullName returns the namespace/name identifier with no default namespace
// (a bare name when the namespace is unset). This is the identifier Vigil keys
// its skills allowlist on.
func (s *Skill) FullName() string {
	if s.Namespace != "" {
		return s.Namespace + "/" + s.Name
	}
	return s.Name
}
