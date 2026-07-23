package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PodmanRuntime implements Runtime using the podman CLI.
type PodmanRuntime struct{}

func (p *PodmanRuntime) Name() string { return "podman" }

func (p *PodmanRuntime) IsAvailable() bool { return checkAvailable("podman") }

func (p *PodmanRuntime) Build(ctx context.Context, opts BuildOpts) error {
	args := buildArgs("podman", opts)
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Dir = opts.ContextDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman build: %w\n%s", err, string(out))
	}
	return nil
}

func (p *PodmanRuntime) Run(ctx context.Context, opts RunOpts) (string, error) {
	args := runArgs("podman", opts)
	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman run: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *PodmanRuntime) Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "podman", "stop", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman stop: %w\n%s", err, string(out))
	}
	return nil
}

func (p *PodmanRuntime) Remove(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman rm: %w\n%s", err, string(out))
	}
	return nil
}

func (p *PodmanRuntime) Exec(ctx context.Context, containerID string, command []string) (string, error) {
	args := append([]string{"exec", containerID}, command...)
	cmd := exec.CommandContext(ctx, "podman", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("podman exec: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}
