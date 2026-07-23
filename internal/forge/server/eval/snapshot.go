package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// readGoldenFile reads and returns the content of a golden file.
func readGoldenFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read golden file %s: %w", path, err)
	}
	return data, nil
}

// jsonEqual compares two JSON byte slices for semantic equality.
// Both values are unmarshalled and re-marshalled to normalize formatting.
func jsonEqual(a, b []byte) bool {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}

	na, err := json.Marshal(va)
	if err != nil {
		return false
	}
	nb, err := json.Marshal(vb)
	if err != nil {
		return false
	}

	return string(na) == string(nb)
}

// UpdateSnapshots writes current tool call results as golden files.
// The results map keys are scenario names, values are the tool call output data.
func UpdateSnapshots(suite *Suite, results map[string]any, baseDir string) error {
	for _, scenario := range suite.Scenarios {
		for _, a := range scenario.Assertions {
			if a.Type != "golden_file" {
				continue
			}
			goldenPath, ok := a.Expected.(string)
			if !ok {
				continue
			}

			data, exists := results[scenario.Name]
			if !exists {
				continue
			}

			out, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal golden for %s: %w", scenario.Name, err)
			}

			fullPath := goldenPath
			if !filepath.IsAbs(fullPath) {
				fullPath = filepath.Join(baseDir, fullPath)
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("create golden dir: %w", err)
			}

			if err := os.WriteFile(fullPath, append(out, '\n'), 0o644); err != nil {
				return fmt.Errorf("write golden file %s: %w", fullPath, err)
			}
		}
	}

	return nil
}
