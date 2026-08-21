package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/dashboard"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/console"
	"github.com/ceasarb/trovery-tools/internal/forge/workspace"
	"github.com/spf13/cobra"
)

var dashboardPort int

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start the Trovery Forge dashboard",
	Long: console.HeaderStyle.Render("Dashboard") + "\n\n" +
		"Launch the web dashboard for viewing servers, agents, evals, and sessions.",
	RunE: runDashboard,
}

func init() {
	dashboardCmd.Flags().IntVar(&dashboardPort, "port", 3000, "port to serve on")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Find(cwd)
	if err != nil {
		return err
	}
	if ws == nil {
		console.Error("Not inside an Trovery Forge workspace.")
		console.Dim("Run 'trove forge init <name>' to create one, or cd into an existing workspace.")
		return fmt.Errorf("workspace not found")
	}

	// Ensure .trove/forge directory exists
	troveDir := ws.Root + "/.trove/forge"
	if err := os.MkdirAll(troveDir, 0o755); err != nil {
		return fmt.Errorf("create .trove/forge directory: %w", err)
	}

	srv, err := dashboard.New(dashboardPort, ws.Root)
	if err != nil {
		return fmt.Errorf("create dashboard server: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d", dashboardPort)
	console.Header("Trovery Forge Dashboard")
	console.Info(fmt.Sprintf("Serving at %s", url))
	console.Dim("Press Ctrl+C to stop")

	// Try to open browser
	openBrowser(url)

	// Graceful shutdown on interrupt
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		console.Dim("\nShutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return
	}
	exec.Command(cmd, url).Start()
}
