package policy

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
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

// ScanContent scans a string for secret patterns (for testing).
func (s *SecretsScanner) ScanContent(content, filePath string) []session.PolicyViolation {
	var violations []session.PolicyViolation

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for i, re := range s.patterns {
			if re.MatchString(line) {
				violations = append(violations, session.PolicyViolation{
					Rule:     "secrets.block_patterns",
					Severity: "error",
					Message:  fmt.Sprintf("Possible secret detected: pattern %q matched", s.names[i]),
					File:     filePath,
					Line:     lineNum,
					Pattern:  s.names[i],
				})
			}
		}
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
