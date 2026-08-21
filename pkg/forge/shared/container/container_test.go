package container

import (
	"testing"
)

func TestBuildArgs(t *testing.T) {
	opts := BuildOpts{
		ContextDir: "/tmp/test",
		Dockerfile: "Dockerfile.trove",
		Tag:        "test-image:latest",
		Labels:     map[string]string{"app": "test"},
		BuildArgs:  map[string]string{"VERSION": "1.0"},
	}

	args := buildArgs("docker", opts)

	// Verify essential flags are present
	assertContains(t, args, "build")
	assertContains(t, args, "-f")
	assertContains(t, args, "Dockerfile.trove")
	assertContains(t, args, "-t")
	assertContains(t, args, "test-image:latest")
	assertContains(t, args, "--label")
	assertContains(t, args, "app=test")
	assertContains(t, args, "--build-arg")
	assertContains(t, args, "VERSION=1.0")
	assertContains(t, args, ".")
}

func TestBuildArgsMinimal(t *testing.T) {
	opts := BuildOpts{
		ContextDir: "/tmp/test",
	}

	args := buildArgs("docker", opts)

	assertContains(t, args, "build")
	assertContains(t, args, ".")
	assertNotContains(t, args, "-f")
	assertNotContains(t, args, "-t")
}

func TestRunArgs(t *testing.T) {
	opts := RunOpts{
		Image:     "test-image",
		Name:      "test-container",
		Detach:    true,
		Remove:    true,
		ReadOnly:  true,
		MemoryMB:  512,
		CPUs:      1.0,
		PidsLimit: 100,
		Ports:     map[string]string{"8080": "8080"},
		Env:       map[string]string{"FOO": "bar"},
	}

	args := runArgs("docker", opts)

	assertContains(t, args, "run")
	assertContains(t, args, "-d")
	assertContains(t, args, "--rm")
	assertContains(t, args, "--read-only")
	assertContains(t, args, "--name")
	assertContains(t, args, "test-container")
	assertContains(t, args, "--memory")
	assertContains(t, args, "512m")
	assertContains(t, args, "--cpus")
	assertContains(t, args, "1.0")
	assertContains(t, args, "--pids-limit")
	assertContains(t, args, "100")
	assertContains(t, args, "-p")
	assertContains(t, args, "8080:8080")
	assertContains(t, args, "-e")
	assertContains(t, args, "FOO=bar")
	assertContains(t, args, "test-image")
}

func TestRunArgsMinimal(t *testing.T) {
	opts := RunOpts{
		Image: "my-image",
	}

	args := runArgs("docker", opts)

	assertContains(t, args, "run")
	assertContains(t, args, "my-image")
	assertNotContains(t, args, "-d")
	assertNotContains(t, args, "--rm")
	assertNotContains(t, args, "--read-only")
}

func TestRuntimeByName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"docker", "docker", false},
		{"podman", "podman", false},
		{"nerdctl", "nerdctl", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := RuntimeByName(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rt.Name() != tt.want {
				t.Errorf("got name %q, want %q", rt.Name(), tt.want)
			}
		})
	}
}

func TestRuntimeNames(t *testing.T) {
	if (&DockerRuntime{}).Name() != "docker" {
		t.Error("DockerRuntime.Name() should be 'docker'")
	}
	if (&PodmanRuntime{}).Name() != "podman" {
		t.Error("PodmanRuntime.Name() should be 'podman'")
	}
	if (&NerdctlRuntime{}).Name() != "nerdctl" {
		t.Error("NerdctlRuntime.Name() should be 'nerdctl'")
	}
}

// --- helpers ---

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}

func assertNotContains(t *testing.T, args []string, unwanted string) {
	t.Helper()
	for _, a := range args {
		if a == unwanted {
			t.Errorf("args %v should not contain %q", args, unwanted)
			return
		}
	}
}
