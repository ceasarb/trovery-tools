package devserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/server/harness"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/config"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/console"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
	"github.com/fsnotify/fsnotify"
)

// DevServer manages a running MCP server with file watching and REPL.
type DevServer struct {
	cfg      *config.ServerConfig
	dir      string
	extraEnv []string
	client   *harness.Client
	mu       sync.Mutex
	cancel   context.CancelFunc
	tools    []protocol.Tool
}

// Run starts the dev server with file watching and REPL.
func Run(cfg *config.ServerConfig, dir string) error {
	return RunWithEnv(cfg, dir, nil)
}

// RunWithEnv starts the dev server with additional environment variables.
func RunWithEnv(cfg *config.ServerConfig, dir string, extraEnv []string) error {
	ds := &DevServer{cfg: cfg, dir: dir, extraEnv: extraEnv}

	// Start server
	if err := ds.startServer(); err != nil {
		return err
	}
	defer ds.stopServer()

	// Start file watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ds.watchFiles(ctx)

	// Run REPL
	return ds.repl()
}

func (ds *DevServer) startServer() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	parts := strings.Fields(ds.cfg.Server.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty server command")
	}

	ctx, cancel := context.WithCancel(context.Background())
	ds.cancel = cancel

	client, err := harness.StartWithEnv(ctx, parts[0], parts[1:], ds.dir, ds.extraEnv)
	if err != nil {
		cancel()
		return fmt.Errorf("start server: %w", err)
	}

	ds.client = client

	// Discover tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		console.Warning(fmt.Sprintf("Could not list tools: %v", err))
	} else {
		ds.tools = tools
	}

	console.Success(fmt.Sprintf("Server running (%d tools)", len(ds.tools)))
	for _, t := range ds.tools {
		console.Dim(fmt.Sprintf("  %s — %s", t.Name, t.Description))
	}

	return nil
}

func (ds *DevServer) stopServer() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.cancel != nil {
		ds.cancel()
	}
	if ds.client != nil {
		ds.client.Close()
		ds.client = nil
	}
}

func (ds *DevServer) restart() {
	console.Info("Restarting server...")
	ds.stopServer()

	// Brief delay for file system to settle
	time.Sleep(500 * time.Millisecond)

	if err := ds.startServer(); err != nil {
		console.Error(fmt.Sprintf("Restart failed: %v", err))
	}
}

func (ds *DevServer) watchFiles(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		console.Error(fmt.Sprintf("File watcher failed: %v", err))
		return
	}
	defer watcher.Close()

	// Watch project directory recursively
	filepath.Walk(ds.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip hidden dirs and common non-source dirs
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "__pycache__" || base == ".venv" {
				return filepath.SkipDir
			}
			watcher.Add(path)
		}
		return nil
	})

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}
			// Debounce restarts
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(300*time.Millisecond, func() {
				console.Dim(fmt.Sprintf("  Changed: %s", filepath.Base(event.Name)))
				ds.restart()
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			console.Warning(fmt.Sprintf("Watcher error: %v", err))
		}
	}
}

func (ds *DevServer) repl() error {
	fmt.Println()
	console.Info("REPL ready. Commands:")
	console.Dim("  tools                    — list available tools")
	console.Dim("  call <tool> <value>       — invoke with bare value")
	console.Dim("  call <tool> key=val ...   — invoke with key=value pairs")
	console.Dim("  call <tool> {\"k\":\"v\"}     — invoke with JSON")
	console.Dim("  quit                     — exit dev server")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(console.HeaderStyle.Render("trove-forge> "))
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case line == "quit" || line == "exit" || line == "q":
			return nil

		case line == "tools":
			ds.mu.Lock()
			for _, t := range ds.tools {
				console.Dim(fmt.Sprintf("  %s — %s", t.Name, t.Description))
			}
			ds.mu.Unlock()

		case strings.HasPrefix(line, "call "):
			ds.handleCall(line[5:])

		default:
			console.Warning("Unknown command: " + line)
			console.Dim("  Try: tools, call <tool> <args>, quit")
		}
	}

	return nil
}

func (ds *DevServer) handleCall(input string) {
	parts := strings.SplitN(strings.TrimSpace(input), " ", 2)
	toolName := parts[0]

	var args interface{}
	if len(parts) > 1 {
		raw := strings.TrimSpace(parts[1])
		args = ds.parseArgs(toolName, raw)
	}

	ds.mu.Lock()
	client := ds.client
	ds.mu.Unlock()

	if client == nil {
		console.Error("Server not running")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, duration, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		console.Error(fmt.Sprintf("Tool call failed: %v", err))
		return
	}

	console.Dim(fmt.Sprintf("  (%dms)", duration.Milliseconds()))
	for _, c := range result.Content {
		if c.Type == "text" {
			fmt.Println(c.Text)
		}
	}
}

// parseArgs handles three input styles:
//
//	call hello {"name": "me"}       → raw JSON
//	call hello name=me              → key=value pairs
//	call hello me                   → bare value → first param of the tool's schema
func (ds *DevServer) parseArgs(toolName string, raw string) interface{} {
	// Try JSON first
	if strings.HasPrefix(raw, "{") {
		var obj interface{}
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			return obj
		}
	}

	// Try key=value pairs (e.g. name=World city=Paris)
	if strings.Contains(raw, "=") {
		result := map[string]interface{}{}
		for _, pair := range splitRespectingQuotes(raw) {
			k, v, ok := strings.Cut(pair, "=")
			if ok {
				result[k] = unquote(v)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Bare value → map to the first parameter of the tool's schema
	ds.mu.Lock()
	paramName := firstParam(ds.tools, toolName)
	ds.mu.Unlock()

	if paramName != "" {
		return map[string]interface{}{paramName: raw}
	}

	// Last resort: treat as single string argument
	return map[string]interface{}{"input": raw}
}

// firstParam extracts the first property name from a tool's input schema.
func firstParam(tools []protocol.Tool, toolName string) string {
	for _, t := range tools {
		if t.Name != toolName || len(t.InputSchema) == 0 {
			continue
		}
		var schema struct {
			Properties map[string]interface{} `json:"properties"`
			Required   []string               `json:"required"`
		}
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			continue
		}
		// Prefer first required param
		if len(schema.Required) > 0 {
			return schema.Required[0]
		}
		// Otherwise first property
		for k := range schema.Properties {
			return k
		}
	}
	return ""
}

// splitRespectingQuotes splits on spaces but keeps quoted strings together.
func splitRespectingQuotes(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range s {
		if !inQuote && (r == '"' || r == '\'') {
			inQuote = true
			quoteChar = r
			continue
		}
		if inQuote && r == quoteChar {
			inQuote = false
			continue
		}
		if !inQuote && r == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func unquote(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
