package safepath

import (
	"testing"
)

func TestResolve_NormalPath(t *testing.T) {
	got, err := Resolve("/workspace", "agents/assistant")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/workspace/agents/assistant" {
		t.Errorf("got %q", got)
	}
}

func TestResolve_RejectsTraversal(t *testing.T) {
	_, err := Resolve("/workspace", "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestResolve_RejectsAbsolutePath(t *testing.T) {
	_, err := Resolve("/workspace", "/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestResolve_HandlesClean(t *testing.T) {
	got, err := Resolve("/workspace", "agents/../agents/assistant")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/workspace/agents/assistant" {
		t.Errorf("got %q", got)
	}
}

func TestResolve_RejectsDotDotPrefix(t *testing.T) {
	_, err := Resolve("/workspace", "../sibling")
	if err == nil {
		t.Error("expected error for .. prefix")
	}
}

func TestResolve_CurrentDir(t *testing.T) {
	got, err := Resolve("/workspace", ".")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/workspace" {
		t.Errorf("got %q", got)
	}
}
