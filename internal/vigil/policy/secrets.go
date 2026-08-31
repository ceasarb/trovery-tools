package policy

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
	vigilpolicy "github.com/ceasarb/trovery-tools/pkg/vigil/policy"
	"gopkg.in/yaml.v3"
)

// SecretsScanner scans file contents for secret patterns.
type SecretsScanner struct {
	patterns []*regexp.Regexp
	names    []string
}

// NewSecretsScanner compiles patterns from the secrets policy.
func NewSecretsScanner(policy config.SecretsPolicy, projectRoot string) (*SecretsScanner, error) {
	s := &SecretsScanner{}

	for _, p := range policy.BlockPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			slog.Warn("skipping invalid regex pattern", "pattern", p, "error", err)
			continue
		}
		s.patterns = append(s.patterns, re)
		s.names = append(s.names, p)
	}

	// Load custom patterns file if specified.
	if policy.CustomPatternsFile != "" {
		customPath := policy.CustomPatternsFile
		if !filepath.IsAbs(customPath) {
			customPath = filepath.Join(projectRoot, customPath)
		}
		if err := s.loadCustomPatterns(customPath); err != nil {
			return nil, fmt.Errorf("loading custom patterns from %s: %w", customPath, err)
		}
	}

	return s, nil
}

func (s *SecretsScanner) loadCustomPatterns(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var patterns []string
	if err := yaml.Unmarshal(data, &patterns); err != nil {
		return fmt.Errorf("parsing custom patterns file: %w", err)
	}

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn("skipping invalid custom pattern", "pattern", pattern, "error", err)
			continue
		}
		s.patterns = append(s.patterns, re)
		s.names = append(s.names, pattern)
	}

	return nil
}

// PatternCount returns the number of compiled patterns (for testing).
func (s *SecretsScanner) PatternCount() int {
	return len(s.patterns)
}

// ScanFiles scans all changed files for secret patterns.
func (s *SecretsScanner) ScanFiles(changes []session.FileChange, projectRoot string) []session.PolicyViolation {
	var violations []session.PolicyViolation

	for _, fc := range changes {
		if fc.ChangeType == "deleted" {
			continue
		}

		path := filepath.Join(projectRoot, fc.Path)
		fileViolations := s.scanFile(path, fc.Path)
		violations = append(violations, fileViolations...)
	}

	return violations
}

// ScanContent scans a string for secret patterns.
//
// The matching itself lives in pkg/vigil/policy so that the pattern list has
// one implementation rather than two that drift. This wrapper exists for the
// session record's shape, which is Vigil's internal audit format.
func (s *SecretsScanner) ScanContent(content, filePath string) []session.PolicyViolation {
	pub, err := vigilpolicy.NewSecrets(s.names...)
	if err != nil {
		// The patterns compiled once already at construction, so this cannot
		// fail here; returning nothing rather than panicking keeps a scan from
		// taking down an audit.
		return nil
	}
	var violations []session.PolicyViolation
	for _, v := range pub.ScanContent(content, filePath) {
		violations = append(violations, session.PolicyViolation{
			Rule:     v.Rule,
			Severity: string(v.Severity),
			Message:  v.Message,
			File:     v.Source,
			Line:     v.Line,
			Pattern:  v.Pattern,
		})
	}
	return violations
}

func (s *SecretsScanner) scanFile(absPath, relPath string) []session.PolicyViolation {
	// Check if binary by reading first bytes.
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return nil
	}

	// Skip binary files (contains null bytes).
	for _, b := range buf[:n] {
		if b == 0 {
			return nil
		}
	}

	// Re-read full file.
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	return s.ScanContent(string(content), relPath)
}
