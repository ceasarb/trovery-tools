package harness

import (
	"context"
	"os/exec"
	"testing"
)

func lookNode() (string, error) { return exec.LookPath("node") }

// runCapture spawns the same way StartWithOptions does, and reads what the
// process printed. The harness client speaks MCP; this test only needs the
// process's view of its own environment.
func runCapture(t *testing.T, ctx context.Context, command string, args []string, dir string, opts Options) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	applyEnv(cmd, opts)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running probe: %v\n%s", err, out)
	}
	return string(out)
}
