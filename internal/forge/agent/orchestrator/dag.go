package orchestrator

import (
	"fmt"
	"strings"
)

// Node represents an agent in the orchestration DAG.
type Node struct {
	Name      string
	Path      string
	DependsOn []string
}

// DAG represents a directed acyclic graph of agent dependencies.
type DAG struct {
	Nodes  map[string]*Node
	Layers [][]string // topologically sorted execution layers
}

// BuildDAG constructs a DAG from agent references, performing
// topological sort and cycle detection.
func BuildDAG(agents []Node) (*DAG, error) {
	dag := &DAG{Nodes: make(map[string]*Node)}

	// Index nodes
	for i := range agents {
		a := &agents[i]
		if _, exists := dag.Nodes[a.Name]; exists {
			return nil, fmt.Errorf("duplicate agent name: %s", a.Name)
		}
		dag.Nodes[a.Name] = a
	}

	// Validate edges
	for _, node := range dag.Nodes {
		for _, dep := range node.DependsOn {
			if _, exists := dag.Nodes[dep]; !exists {
				return nil, fmt.Errorf("agent %q depends on unknown agent %q", node.Name, dep)
			}
		}
	}

	// Topological sort using Kahn's algorithm
	layers, err := topoSort(dag.Nodes)
	if err != nil {
		return nil, err
	}
	dag.Layers = layers

	return dag, nil
}

// topoSort performs Kahn's algorithm, grouping nodes into parallel execution layers.
func topoSort(nodes map[string]*Node) ([][]string, error) {
	// Compute in-degrees
	inDegree := make(map[string]int)
	for name := range nodes {
		inDegree[name] = 0
	}
	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			inDegree[node.Name]++
			_ = dep // dep is the dependency, node depends on dep
		}
	}

	// Recompute correctly: in-degree = number of dependencies
	for name := range nodes {
		inDegree[name] = len(nodes[name].DependsOn)
	}

	var layers [][]string
	remaining := len(nodes)

	for remaining > 0 {
		// Find all nodes with in-degree 0
		var layer []string
		for name, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, name)
			}
		}

		if len(layer) == 0 {
			// Find cycle participants for error message
			var cycleNodes []string
			for name, deg := range inDegree {
				if deg > 0 {
					cycleNodes = append(cycleNodes, name)
				}
			}
			return nil, fmt.Errorf("dependency cycle detected among agents: %s", strings.Join(cycleNodes, ", "))
		}

		// Remove this layer's nodes
		for _, name := range layer {
			delete(inDegree, name)
			remaining--
		}

		// Decrement in-degree for dependents
		for name := range inDegree {
			node := nodes[name]
			for _, dep := range node.DependsOn {
				for _, done := range layer {
					if dep == done {
						inDegree[name]--
					}
				}
			}
		}

		layers = append(layers, layer)
	}

	return layers, nil
}

// RenderASCII returns an ASCII art representation of the DAG.
func (d *DAG) RenderASCII() string {
	var b strings.Builder

	for i, layer := range d.Layers {
		if i > 0 {
			// Draw arrows from previous layer
			b.WriteString("    │\n    ▼\n")
		}

		b.WriteString(fmt.Sprintf("  Layer %d: ", i+1))
		if len(layer) == 1 {
			b.WriteString(fmt.Sprintf("[%s]", layer[0]))
		} else {
			names := make([]string, len(layer))
			for j, name := range layer {
				names[j] = fmt.Sprintf("[%s]", name)
			}
			b.WriteString(strings.Join(names, "  "))
			b.WriteString("  (parallel)")
		}
		b.WriteString("\n")
	}

	return b.String()
}
