package container

import (
	"context"
	"fmt"
	"os/exec"
)

// Runtime abstracts container operations across Docker, Podman, and nerdctl.
type Runtime interface {
	Name() string
	Build(ctx context.Context, opts BuildOpts) error
	Run(ctx context.Context, opts RunOpts) (string, error) // returns container ID
	Stop(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	IsAvailable() bool
}

// BuildOpts configures a container image build.
type BuildOpts struct {
	ContextDir string
	Dockerfile string
	Tag        string
	Labels     map[string]string
	BuildArgs  map[string]string
}

// RunOpts configures a container run.
type RunOpts struct {
	Image    string
	Name     string
	Ports    map[string]string // host:container
	Env      map[string]string
	Volumes  map[string]string // host:container
	Labels   map[string]string
	MemoryMB int
	CPUs     float64
	PidsLimit int
	ReadOnly bool
	Remove   bool // --rm
	Detach   bool
}

// DetectRuntime probes for docker, podman, and nerdctl in order, returning
// the first available runtime.
func DetectRuntime() (Runtime, error) {
	runtimes := []Runtime{
		&DockerRuntime{},
		&PodmanRuntime{},
		&NerdctlRuntime{},
	}

	for _, rt := range runtimes {
		if rt.IsAvailable() {
			return rt, nil
		}
	}

	return nil, fmt.Errorf("no container runtime found (tried docker, podman, nerdctl)")
}

// RuntimeByName returns a specific runtime by name, or an error if unknown.
func RuntimeByName(name string) (Runtime, error) {
	switch name {
	case "docker":
		return &DockerRuntime{}, nil
	case "podman":
		return &PodmanRuntime{}, nil
	case "nerdctl":
		return &NerdctlRuntime{}, nil
	default:
		return nil, fmt.Errorf("unknown container runtime: %s (expected docker, podman, or nerdctl)", name)
	}
}

// checkAvailable returns true if the given binary is on PATH and responds
// to "version --format json".
func checkAvailable(binary string) bool {
	cmd := exec.Command(binary, "version", "--format", "json")
	return cmd.Run() == nil
}
