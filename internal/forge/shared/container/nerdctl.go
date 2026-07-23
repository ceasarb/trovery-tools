package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NerdctlRuntime implements Runtime using the nerdctl CLI.
type NerdctlRuntime struct{}

func (n *NerdctlRuntime) Name() string { return "nerdctl" }

func (n *NerdctlRuntime) IsAvailable() bool { return checkAvailable("nerdctl") }

func (n *NerdctlRuntime) Build(ctx context.Context, opts BuildOpts) error {
	args := buildArgs("nerdctl", opts)
	cmd := exec.CommandContext(ctx, "nerdctl", args...)
	cmd.Dir = opts.ContextDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl build: %w\n%s", err, string(out))
	}
	return nil
}

func (n *NerdctlRuntime) Run(ctx context.Context, opts RunOpts) (string, error) {
	args := runArgs("nerdctl", opts)
	cmd := exec.CommandContext(ctx, "nerdctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nerdctl run: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (n *NerdctlRuntime) Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "nerdctl", "stop", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl stop: %w\n%s", err, string(out))
	}
	return nil
}

func (n *NerdctlRuntime) Remove(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "nerdctl", "rm", "-f", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl rm: %w\n%s", err, string(out))
	}
	return nil
}

func (n *NerdctlRuntime) Exec(ctx context.Context, containerID string, command []string) (string, error) {
	args := append([]string{"exec", containerID}, command...)
	cmd := exec.CommandContext(ctx, "nerdctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nerdctl exec: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}
