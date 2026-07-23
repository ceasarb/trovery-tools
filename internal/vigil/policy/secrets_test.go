package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

func defaultSecretsPolicy() config.SecretsPolicy {
	return config.DefaultConfig().Policies.Secrets
}

func TestSecretsScanner_DetectsAWSKey(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`AWS_SECRET_ACCESS_KEY = AKIA`+`IOSFODNN7EXAMPLE`,
		"config.go",
	)
	if len(violations) == 0 {
		t.Error("expected AWS key violation")
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", violations[0].Severity)
	}
}

func TestSecretsScanner_DetectsGitHubPAT(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`token := "ghp_`+`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"`,
		"auth.go",
	)
	if len(violations) == 0 {
		t.Error("expected GitHub PAT violation")
	}
}

func TestSecretsScanner_DetectsPassword(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`password = mysecretpassword123`,
		"config.go",
	)
	if len(violations) == 0 {
		t.Error("expected password violation")
	}
}

func TestSecretsScanner_DetectsBearerToken(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
		"handler.go",
	)
	if len(violations) == 0 {
		t.Error("expected bearer token violation")
	}
}

func TestSecretsScanner_NoFalsePositive(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`func main() {
	fmt.Println("Hello, world!")
	x := 42
}`,
		"main.go",
	)
	if len(violations) != 0 {
		t.Errorf("expected no violations on safe content, got %d", len(violations))
	}
}

func TestSecretsScanner_LineNumbers(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		"line 1\nline 2\npassword = secret123\nline 4\n",
		"config.go",
	)
	if len(violations) == 0 {
		t.Fatal("expected violation")
	}
	if violations[0].Line != 3 {
		t.Errorf("expected line 3, got %d", violations[0].Line)
	}
}

func TestSecretsScanner_ScanFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.go"), []byte("password = secret123\n"), 0644)

	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), dir)
	changes := []session.FileChange{{Path: "bad.go", ChangeType: "modified"}}
	violations := scanner.ScanFiles(changes, dir)
	if len(violations) == 0 {
		t.Error("expected violation from file scan")
	}
}

func TestSecretsScanner_SkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a binary file (contains null bytes).
	content := []byte("password = secret123\x00\x00binary data")
	os.WriteFile(filepath.Join(dir, "data.bin"), content, 0644)

	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), dir)
	changes := []session.FileChange{{Path: "data.bin", ChangeType: "modified"}}
	violations := scanner.ScanFiles(changes, dir)
	if len(violations) != 0 {
		t.Errorf("expected no violations for binary file, got %d", len(violations))
	}
}

func TestSecretsScanner_SkipsDeletedFiles(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	changes := []session.FileChange{{Path: "old.go", ChangeType: "deleted"}}
	violations := scanner.ScanFiles(changes, t.TempDir())
	if len(violations) != 0 {
		t.Errorf("expected no violations for deleted files, got %d", len(violations))
	}
}

func TestSecretsScanner_DetectsOpenAIKey(t *testing.T) {
	scanner, _ := NewSecretsScanner(defaultSecretsPolicy(), "")
	violations := scanner.ScanContent(
		`OPENAI_KEY="sk-`+`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"`,
		"env.go",
	)
	if len(violations) == 0 {
		t.Error("expected OpenAI key violation")
	}
}

func TestSecretsScanner_CustomPatternsFile(t *testing.T) {
	dir := t.TempDir()
	patternsFile := filepath.Join(dir, ".vigil-secrets.yaml")
	os.WriteFile(patternsFile, []byte(`- "STRIPE_KEY_[a-z]+_[A-Za-z0-9]+"
- "slack-token-[A-Za-z0-9]+"
`), 0644)

	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = patternsFile

	scanner, err := NewSecretsScanner(policy, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 6 defaults + 2 custom = 8.
	if scanner.PatternCount() != 8 {
		t.Errorf("expected 8 patterns, got %d", scanner.PatternCount())
	}

	// Should detect custom pattern.
	violations := scanner.ScanContent(`key = "STRIPE_KEY_live_abc123XYZ"`, "billing.go")
	if len(violations) == 0 {
		t.Error("expected Stripe key violation from custom pattern")
	}
}

func TestSecretsScanner_CustomPatternsFile_Merge(t *testing.T) {
	dir := t.TempDir()
	patternsFile := filepath.Join(dir, "custom.yaml")
	os.WriteFile(patternsFile, []byte(`- "CUSTOM_SECRET_[0-9]+"
`), 0644)

	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = patternsFile

	scanner, err := NewSecretsScanner(policy, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Built-in patterns still work.
	v1 := scanner.ScanContent("password = secret123", "a.go")
	if len(v1) == 0 {
		t.Error("expected built-in pattern to still match")
	}

	// Custom patterns also work.
	v2 := scanner.ScanContent("key = CUSTOM_SECRET_42", "b.go")
	if len(v2) == 0 {
		t.Error("expected custom pattern to match")
	}
}

func TestSecretsScanner_CustomPatternsFile_BadRegex(t *testing.T) {
	dir := t.TempDir()
	patternsFile := filepath.Join(dir, "bad.yaml")
	os.WriteFile(patternsFile, []byte(`- "[invalid(regex"
- "valid_pattern_[0-9]+"
`), 0644)

	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = patternsFile

	scanner, err := NewSecretsScanner(policy, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 6 defaults + 1 valid custom = 7 (bad regex skipped with warning).
	if scanner.PatternCount() != 7 {
		t.Errorf("expected 7 patterns (bad regex skipped), got %d", scanner.PatternCount())
	}

	// Valid custom pattern still works.
	violations := scanner.ScanContent("valid_pattern_999", "a.go")
	if len(violations) == 0 {
		t.Error("expected valid custom pattern to match")
	}
}

func TestSecretsScanner_CustomPatternsFile_NotFound(t *testing.T) {
	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = "/nonexistent/patterns.yaml"

	_, err := NewSecretsScanner(policy, "")
	if err == nil {
		t.Error("expected error for missing custom patterns file")
	}
}

func TestSecretsScanner_CustomPatternsFile_Empty(t *testing.T) {
	dir := t.TempDir()
	patternsFile := filepath.Join(dir, "empty.yaml")
	os.WriteFile(patternsFile, []byte("[]"), 0644)

	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = patternsFile

	scanner, err := NewSecretsScanner(policy, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the 6 defaults.
	if scanner.PatternCount() != 6 {
		t.Errorf("expected 6 patterns (empty custom), got %d", scanner.PatternCount())
	}
}

func TestSecretsScanner_CustomPatternsFile_RelativePath(t *testing.T) {
	dir := t.TempDir()
	patternsFile := filepath.Join(dir, ".vigil-secrets.yaml")
	os.WriteFile(patternsFile, []byte(`- "RELATIVE_TEST_[0-9]+"
`), 0644)

	policy := defaultSecretsPolicy()
	policy.CustomPatternsFile = ".vigil-secrets.yaml" // Relative path.

	scanner, err := NewSecretsScanner(policy, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if scanner.PatternCount() != 7 {
		t.Errorf("expected 7 patterns, got %d", scanner.PatternCount())
	}
}
