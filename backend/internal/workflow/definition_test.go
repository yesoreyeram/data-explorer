package workflow

import "testing"

func TestTopologicalOrderSimpleChain(t *testing.T) {
	def := Definition{
		Nodes: []Node{{ID: "a", Type: NodeTypeSource}, {ID: "b", Type: NodeTypeTransform}, {ID: "c", Type: NodeTypeOutput}},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "b"}, {ID: "e2", Source: "b", Target: "c"}},
	}

	order, err := TopologicalOrder(def)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if !(pos["a"] < pos["b"] && pos["b"] < pos["c"]) {
		t.Fatalf("expected order a < b < c, got %v", order)
	}
}

func TestTopologicalOrderDetectsCycle(t *testing.T) {
	def := Definition{
		Nodes: []Node{{ID: "a", Type: NodeTypeTransform}, {ID: "b", Type: NodeTypeTransform}},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "b"}, {ID: "e2", Source: "b", Target: "a"}},
	}

	if _, err := TopologicalOrder(def); err == nil {
		t.Fatal("expected cycle to be detected")
	}
}

func TestValidateRejectsUnknownNodeType(t *testing.T) {
	def := Definition{Nodes: []Node{{ID: "a", Type: "bogus"}}}
	if err := def.Validate(); err == nil {
		t.Fatal("expected unknown node type to be rejected")
	}
}

func TestValidateRejectsDuplicateNodeIDs(t *testing.T) {
	def := Definition{Nodes: []Node{{ID: "a", Type: NodeTypeSource}, {ID: "a", Type: NodeTypeOutput}}}
	if err := def.Validate(); err == nil {
		t.Fatal("expected duplicate node id to be rejected")
	}
}

func TestValidateRejectsDanglingEdge(t *testing.T) {
	def := Definition{
		Nodes: []Node{{ID: "a", Type: NodeTypeSource}},
		Edges: []Edge{{ID: "e1", Source: "a", Target: "missing"}},
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected dangling edge to be rejected")
	}
}
