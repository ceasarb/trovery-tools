package container

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DockerRuntime implements Runtime using the docker CLI.
type DockerRuntime struct{}

func (d *DockerRuntime) Name() string { return "docker" }

func (d *DockerRuntime) IsAvailable() bool { return checkAvailable("docker") }

func (d *DockerRuntime) Build(ctx context.Context, opts BuildOpts) error {
	args := buildArgs("docker", opts)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.ContextDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}

func (d *DockerRuntime) Run(ctx context.Context, opts RunOpts) (string, error) {
	args := runArgs("docker", opts)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *DockerRuntime) Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop: %w\n%s", err, string(out))
	}
	return nil
}

func (d *DockerRuntime) Remove(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm: %w\n%s", err, string(out))
	}
	return nil
}

func (d *DockerRuntime) Exec(ctx context.Context, containerID string, command []string) (string, error) {
	args := append([]string{"exec", containerID}, command...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker exec: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}

// buildArgs constructs the argument list for a "build" command.
func buildArgs(_ string, opts BuildOpts) []string {
	args := []string{"build"}

	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}
	if opts.Tag != "" {
		args = append(args, "-t", opts.Tag)
	}
	for k, v := range opts.Labels {
		args = append(args, "--label", k+"="+v)
	}
	for k, v := range opts.BuildArgs {
		args = append(args, "--build-arg", k+"="+v)
	}

	args = append(args, ".")
	return args
}

// runArgs constructs the argument list for a "run" command.
func runArgs(_ string, opts RunOpts) []string {
	args := []string{"run"}

	if opts.Detach {
		args = append(args, "-d")
	}
	if opts.Remove {
		args = append(args, "--rm")
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.ReadOnly {
		args = append(args, "--read-only")
	}
	if opts.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", opts.MemoryMB))
	}
	if opts.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.1f", opts.CPUs))
	}
	if opts.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", opts.PidsLimit))
	}
	for host, container := range opts.Ports {
		args = append(args, "-p", host+":"+container)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}
	for host, container := range opts.Volumes {
		args = append(args, "-v", host+":"+container)
	}
	for k, v := range opts.Labels {
		args = append(args, "--label", k+"="+v)
	}

	args = append(args, opts.Image)
	return args
}
