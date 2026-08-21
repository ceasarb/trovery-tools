package palette

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	paletteclient "github.com/ceasarb/trovery-tools/internal/forge/palette/go-client"
	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

// ViewerProcess wraps a running Palette viewer subprocess.
type ViewerProcess struct {
	cmd  *exec.Cmd
	port int
}

// StartViewer launches the Palette viewer in dev mode as a subprocess.
// It runs `npx palette-ui dev --port {port}` from the palette/server directory.
func StartViewer(paletteServerDir string, port int) (*ViewerProcess, error) {
	cmd := exec.Command("npx", "palette-ui", "start", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = paletteServerDir
	// Suppress viewer process output — it's noisy and mangles the terminal prompt
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start palette viewer: %w", err)
	}

	return &ViewerProcess{cmd: cmd, port: port}, nil
}

// WaitReady polls the viewer's HTTP endpoint until it responds or the timeout expires.
func WaitReady(port int, timeout time.Duration) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("palette viewer not ready on port %d after %s", port, timeout)
}

// OpenBrowser opens the given URL in the default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS for browser open: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// Stop shuts down the viewer process gracefully.
func (v *ViewerProcess) Stop() {
	if v.cmd == nil || v.cmd.Process == nil {
		return
	}

	// Send interrupt signal
	v.cmd.Process.Signal(os.Interrupt)

	// Wait with timeout
	done := make(chan error, 1)
	go func() { done <- v.cmd.Wait() }()

	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		// Force kill
		v.cmd.Process.Kill()
		<-done
	}
}

// Session manages the full Palette lifecycle for an agent chat session:
// viewer process, WebSocket client, and tool provider.
type Session struct {
	Viewer   *ViewerProcess
	Client   *paletteclient.Client
	Provider *ToolProvider
	Config   *agentcfg.PaletteConfig
}

// StartSession initializes the full Palette stack for an agent chat.
// It starts the viewer, waits for it, connects the Go client, and creates the tool provider.
// paletteServerDir is the path to the palette/server/ directory.
func StartSession(ctx context.Context, cfg *agentcfg.PaletteConfig, paletteServerDir string) (*Session, error) {
	port := cfg.EffectivePort()

	// Start viewer
	viewer, err := StartViewer(paletteServerDir, port)
	if err != nil {
		return nil, err
	}

	// Wait for viewer to be ready
	if err := WaitReady(port, 15*time.Second); err != nil {
		viewer.Stop()
		return nil, err
	}

	// Connect Go WebSocket client
	wsURL := fmt.Sprintf("ws://localhost:%d", port)
	client := paletteclient.New(wsURL)

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx); err != nil {
		viewer.Stop()
		return nil, fmt.Errorf("palette WS connect: %w", err)
	}

	// Parse prompt timeout
	promptTimeout := 5 * time.Minute
	if cfg.PromptTimeout != "" && cfg.PromptTimeout != "0" {
		if d, err := time.ParseDuration(cfg.PromptTimeout); err == nil {
			promptTimeout = d
		}
	} else if cfg.PromptTimeout == "0" {
		promptTimeout = 0
	}

	provider := NewToolProvider(client, promptTimeout)

	// Note: browser open is deferred — caller should call session.OpenBrowserIfEnabled()
	// after sending agent info so the viewer has metadata on first load.

	return &Session{
		Viewer:   viewer,
		Client:   client,
		Provider: provider,
		Config:   cfg,
	}, nil
}

// OpenBrowserIfEnabled opens the viewer in the default browser if auto_open is configured.
func (s *Session) OpenBrowserIfEnabled() {
	if s.Config != nil && s.Config.AutoOpen {
		viewerURL := fmt.Sprintf("http://localhost:%d", s.Config.EffectivePort())
		OpenBrowser(viewerURL)
	}
}

// Close shuts down the entire Palette session.
func (s *Session) Close() {
	if s.Client != nil {
		s.Client.Close()
	}
	if s.Viewer != nil {
		s.Viewer.Stop()
	}
}
