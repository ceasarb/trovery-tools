package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/shared/delegation"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// Client is an MCP stdio client that talks to a server subprocess.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	nextID int
}

// Start spawns the server process and initializes the MCP connection.
func Start(ctx context.Context, command string, args []string, dir string) (*Client, error) {
	return StartWithEnv(ctx, command, args, dir, nil)
}

// StartWithEnv spawns the server process with additional environment variables.
// Extra env vars are appended to the current process environment (overriding duplicates).
func StartWithEnv(ctx context.Context, command string, args []string, dir string, extraEnv []string) (*Client, error) {
	return StartWithOptions(ctx, command, args, dir, Options{Env: extraEnv})
}

// Options configures how a server subprocess is spawned.
type Options struct {
	// Env is environment for the server process.
	Env []string

	// CleanEnv replaces the parent environment rather than extending it.
	//
	// A sandboxed server is granted a set of capabilities, and the ambient
	// environment is not among them: the process that spawns a tool server may
	// hold API keys, tokens and session state that the server was never
	// declared to need. Extending is the right default for a local development
	// harness and the wrong one for anything running untrusted code, so the
	// caller states which it is.
	//
	// Env is the whole environment when this is set. A server that needs PATH
	// or HOME must be given them explicitly, which is the point.
	CleanEnv bool
}

// applyEnv sets a command's environment according to the options.
func applyEnv(cmd *exec.Cmd, opts Options) {
	switch {
	case opts.CleanEnv:
		// Never nil: a nil cmd.Env means "inherit", which is the opposite of
		// what was asked for. An empty non-nil slice means "empty environment".
		cmd.Env = append([]string{}, opts.Env...)
	case len(opts.Env) > 0:
		cmd.Env = append(os.Environ(), opts.Env...)
	}
}

// StartWithOptions spawns the server process under the given options.
func StartWithOptions(ctx context.Context, command string, args []string, dir string, opts Options) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir

	applyEnv(cmd, opts)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start server: %w", err)
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
		nextID: 1,
	}

	// Send initialize
	_, err = c.Initialize(ctx)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification
	c.sendNotification("notifications/initialized", nil)

	return c, nil
}

// Initialize sends the MCP initialize request.
func (c *Client) Initialize(ctx context.Context) (*protocol.InitializeResult, error) {
	params := protocol.InitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo: protocol.ClientInfo{
			Name:    "trove-forge-test",
			Version: "0.1.0",
		},
	}

	resp, err := c.send(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal init result: %w", err)
	}

	return &result, nil
}

// ListTools sends tools/list and returns available tools.
func (c *Client) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	resp, err := c.send(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result protocol.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	return result.Tools, nil
}

// CallTool invokes a tool and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	params := protocol.ToolCallParams{
		Name:      name,
		Arguments: args,
	}

	// Attach a per-request delegated-identity assertion as MCP `_meta`, never as
	// an argument (ADR-008). Forge only transports it; the tool server verifies.
	if assertion, ok := delegation.OnBehalfOf(ctx); ok {
		params.Meta = map[string]any{delegation.MetaKey: assertion}
	}

	start := time.Now()
	resp, err := c.send(ctx, "tools/call", params)
	elapsed := time.Since(start)

	if err != nil {
		return nil, elapsed, err
	}

	if resp.Error != nil {
		return nil, elapsed, fmt.Errorf("tool error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	var result protocol.ToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, elapsed, fmt.Errorf("unmarshal tool result: %w", err)
	}

	return &result, elapsed, nil
}

// Close shuts down the server process.
func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

// send writes a newline-delimited JSON-RPC message and reads the response.
// Skips notification messages (no ID) that servers may emit before the response.
func (c *Client) send(ctx context.Context, method string, params interface{}) (*protocol.Response, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write newline-delimited JSON-RPC
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response lines, skipping notifications
	type result struct {
		resp *protocol.Response
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		for {
			line, err := c.reader.ReadString('\n')
			if err != nil {
				ch <- result{nil, fmt.Errorf("read response: %w", err)}
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Try to parse as a response (has "id" field)
			var msg json.RawMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue // skip unparseable lines
			}

			// Check if this is a notification (no "id" field) or a response
			var check struct {
				ID *int `json:"id"`
			}
			json.Unmarshal([]byte(line), &check)
			if check.ID == nil {
				continue // skip notifications
			}

			var resp protocol.Response
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				ch <- result{nil, fmt.Errorf("unmarshal response: %w", err)}
				return
			}
			ch <- result{&resp, nil}
			return
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.resp, r.err
	}
}

// sendNotification sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) sendNotification(method string, params interface{}) {
	type notification struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}

	data, _ := json.Marshal(notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
	fmt.Fprintf(c.stdin, "%s\n", data)
}
