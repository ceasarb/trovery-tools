package servermgr

import (
	"path"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// FilterTools applies include/exclude patterns to a tool list.
// If include is set, only matching tools are kept. Exclude is applied after include.
// Patterns support glob matching via path.Match (e.g., "search_*", "get_*").
func FilterTools(tools []protocol.Tool, filter *agentcfg.ToolFilter) []protocol.Tool {
	if filter == nil {
		return tools
	}

	var result []protocol.Tool

	for _, t := range tools {
		// Apply include filter — if set, tool must match at least one pattern
		if len(filter.Include) > 0 {
			if !matchesAny(t.Name, filter.Include) {
				continue
			}
		}

		// Apply exclude filter — if tool matches, skip it
		if len(filter.Exclude) > 0 {
			if matchesAny(t.Name, filter.Exclude) {
				continue
			}
		}

		result = append(result, t)
	}

	return result
}

// matchesAny checks if name matches any of the given glob patterns.
func matchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, name)
		if err != nil {
			// Invalid pattern — treat as literal exact match
			if name == pattern {
				return true
			}
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
