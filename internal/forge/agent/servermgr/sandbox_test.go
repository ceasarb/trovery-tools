package servermgr

import (
	"testing"

	"github.com/ceasarb/trovery-tools/internal/forge/server/sandbox"
)

func TestBuildRunArgs_StrictPolicy(t *testing.T) {
	policy := &sandbox.SecurityPolicy{
		Name:      "strict",
		Network:   false,
		ReadOnly:  true,
		MemoryMB:  256,
		CPUs:      0.5,
		PidsLimit: 50,
	}

	args := buildRunArgs("docker", "trove-forge-serve-weather", "weather", policy)

	assertContains(t, args, "run")
	assertContains(t, args, "-i")
	assertContains(t, args, "--rm")
	assertContains(t, args, "--memory=256m")
	assertContains(t, args, "--cpus=0.5")
	assertContains(t, args, "--pids-limit=50")
	assertContains(t, args, "--read-only")
	assertContains(t, args, "--network=none")
	assertContains(t, args, "--cap-drop=ALL")
	assertContains(t, args, "--security-opt=no-new-privileges:true")
	assertNotContains(t, args, "--cap-add=NET_ADMIN")

	// Image tag should be last
	if args[len(args)-1] != "trove-forge-serve-weather" {
		t.Errorf("image tag should be last argument, got %s", args[len(args)-1])
	}
}

func TestBuildRunArgs_StandardPolicy(t *testing.T) {
	policy := &sandbox.SecurityPolicy{
		Name:      "standard",
		Network:   true,
		ReadOnly:  false,
		MemoryMB:  512,
		CPUs:      1.0,
		PidsLimit: 100,
	}

	args := buildRunArgs("docker", "trove-forge-serve-api", "api", policy)

	assertContains(t, args, "--memory=512m")
	assertContains(t, args, "--cpus=1.0")
	assertNotContains(t, args, "--network=none")
	assertNotContains(t, args, "--read-only")
	assertNotContains(t, args, "--cap-add=NET_ADMIN")
}

func TestBuildRunArgs_DomainFilterPolicy(t *testing.T) {
	policy := &sandbox.SecurityPolicy{
		Name:      "custom",
		Network:   true,
		Domains:   []string{"api.example.com", "*.github.com"},
		ReadOnly:  false,
		MemoryMB:  512,
		CPUs:      1.0,
		PidsLimit: 100,
	}

	args := buildRunArgs("docker", "trove-forge-serve-api", "api", policy)

	// Domain filtering requires NET_ADMIN for iptables
	assertContains(t, args, "--cap-add=NET_ADMIN")
	assertNotContains(t, args, "--network=none")
}

func TestBuildRunArgs_ReadOnlyWithTmpfs(t *testing.T) {
	policy := &sandbox.SecurityPolicy{
		Name:     "strict",
		ReadOnly: true,
		MemoryMB: 256,
		CPUs:     0.5,
	}

	args := buildRunArgs("docker", "trove-forge-serve-test", "test", policy)

	assertContains(t, args, "--read-only")
	assertContains(t, args, "--tmpfs")

	// Find the tmpfs value
	for i, a := range args {
		if a == "--tmpfs" && i+1 < len(args) {
			if args[i+1] != "/tmp:rw,noexec,nosuid,size=64m" {
				t.Errorf("tmpfs value = %s, want /tmp:rw,noexec,nosuid,size=64m", args[i+1])
			}
			return
		}
	}
	t.Error("--tmpfs flag found but no value after it")
}

func assertContains(t *testing.T, args []string, target string) {
	t.Helper()
	for _, a := range args {
		if a == target {
			return
		}
	}
	t.Errorf("args should contain %q, got %v", target, args)
}

func assertNotContains(t *testing.T, args []string, target string) {
	t.Helper()
	for _, a := range args {
		if a == target {
			t.Errorf("args should not contain %q", target)
			return
		}
	}
}
