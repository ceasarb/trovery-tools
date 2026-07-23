package adapters

// Registry maps tool names to their adapter implementations.
var Registry = map[string]Adapter{
	"claude-code":  &ClaudeCodeAdapter{},
	"codex":        &CodexAdapter{},
	"cursor":       &CursorAdapter{},
	"forge-agent":  &ForgeAdapter{},
}

// ResolveAdapter returns the appropriate adapter for a tool name.
// If the tool is not in the registry and a command is provided, returns a GenericAdapter.
func ResolveAdapter(name string, command []string) Adapter {
	if adapter, ok := Registry[name]; ok {
		return adapter
	}
	if len(command) > 0 {
		return NewGenericAdapter(command)
	}
	return nil
}
