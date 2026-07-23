package orchestrator

import (
	"strings"
	"testing"
)

func TestBuildDAGLinear(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/agents/a"},
		{Name: "b", Path: "/agents/b", DependsOn: []string{"a"}},
		{Name: "c", Path: "/agents/c", DependsOn: []string{"b"}},
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(dag.Layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(dag.Layers))
	}

	// Layer 1: a, Layer 2: b, Layer 3: c
	if dag.Layers[0][0] != "a" {
		t.Fatalf("expected layer 1 = [a], got %v", dag.Layers[0])
	}
	if dag.Layers[1][0] != "b" {
		t.Fatalf("expected layer 2 = [b], got %v", dag.Layers[1])
	}
	if dag.Layers[2][0] != "c" {
		t.Fatalf("expected layer 3 = [c], got %v", dag.Layers[2])
	}
}

func TestBuildDAGParallel(t *testing.T) {
	nodes := []Node{
		{Name: "research", Path: "/agents/research"},
		{Name: "analyze", Path: "/agents/analyze"},
		{Name: "synthesize", Path: "/agents/synthesize", DependsOn: []string{"research", "analyze"}},
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(dag.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(dag.Layers))
	}

	// Layer 1: research + analyze (parallel), Layer 2: synthesize
	if len(dag.Layers[0]) != 2 {
		t.Fatalf("expected 2 agents in layer 1, got %d", len(dag.Layers[0]))
	}
	if len(dag.Layers[1]) != 1 || dag.Layers[1][0] != "synthesize" {
		t.Fatalf("expected layer 2 = [synthesize], got %v", dag.Layers[1])
	}
}

func TestBuildDAGSingleNode(t *testing.T) {
	nodes := []Node{
		{Name: "solo", Path: "/agents/solo"},
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(dag.Layers) != 1 || len(dag.Layers[0]) != 1 {
		t.Fatalf("expected 1 layer with 1 node, got %v", dag.Layers)
	}
}

func TestBuildDAGAllParallel(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/agents/a"},
		{Name: "b", Path: "/agents/b"},
		{Name: "c", Path: "/agents/c"},
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(dag.Layers) != 1 {
		t.Fatalf("expected 1 layer (all parallel), got %d", len(dag.Layers))
	}
	if len(dag.Layers[0]) != 3 {
		t.Fatalf("expected 3 agents in layer 1, got %d", len(dag.Layers[0]))
	}
}

func TestBuildDAGCycleDetection(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/agents/a", DependsOn: []string{"b"}},
		{Name: "b", Path: "/agents/b", DependsOn: []string{"a"}},
	}

	_, err := BuildDAG(nodes)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error message, got: %v", err)
	}
}

func TestBuildDAGThreeNodeCycle(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/a", DependsOn: []string{"c"}},
		{Name: "b", Path: "/b", DependsOn: []string{"a"}},
		{Name: "c", Path: "/c", DependsOn: []string{"b"}},
	}

	_, err := BuildDAG(nodes)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestBuildDAGUnknownDependency(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/a", DependsOn: []string{"nonexistent"}},
	}

	_, err := BuildDAG(nodes)
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected error to mention nonexistent, got: %v", err)
	}
}

func TestBuildDAGDuplicateName(t *testing.T) {
	nodes := []Node{
		{Name: "a", Path: "/a"},
		{Name: "a", Path: "/a2"},
	}

	_, err := BuildDAG(nodes)
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}
}

func TestBuildDAGDiamondShape(t *testing.T) {
	// Diamond: a -> b, a -> c, b -> d, c -> d
	nodes := []Node{
		{Name: "a", Path: "/a"},
		{Name: "b", Path: "/b", DependsOn: []string{"a"}},
		{Name: "c", Path: "/c", DependsOn: []string{"a"}},
		{Name: "d", Path: "/d", DependsOn: []string{"b", "c"}},
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(dag.Layers) != 3 {
		t.Fatalf("expected 3 layers for diamond, got %d", len(dag.Layers))
	}
	if len(dag.Layers[0]) != 1 { // a
		t.Fatalf("expected 1 in layer 1, got %d", len(dag.Layers[0]))
	}
	if len(dag.Layers[1]) != 2 { // b, c parallel
		t.Fatalf("expected 2 in layer 2, got %d", len(dag.Layers[1]))
	}
	if len(dag.Layers[2]) != 1 { // d
		t.Fatalf("expected 1 in layer 3, got %d", len(dag.Layers[2]))
	}
}

func TestRenderASCII(t *testing.T) {
	nodes := []Node{
		{Name: "research", Path: "/research"},
		{Name: "analyze", Path: "/analyze"},
		{Name: "synthesize", Path: "/synthesize", DependsOn: []string{"research", "analyze"}},
	}

	dag, _ := BuildDAG(nodes)
	output := dag.RenderASCII()

	if !strings.Contains(output, "Layer 1") {
		t.Fatal("expected Layer 1 in output")
	}
	if !strings.Contains(output, "Layer 2") {
		t.Fatal("expected Layer 2 in output")
	}
	if !strings.Contains(output, "parallel") {
		t.Fatal("expected 'parallel' annotation for multi-agent layer")
	}
	if !strings.Contains(output, "synthesize") {
		t.Fatal("expected 'synthesize' in output")
	}
}
