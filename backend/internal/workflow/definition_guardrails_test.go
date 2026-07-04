package workflow

import "testing"

func TestValidateRejectsTooManyNodes(t *testing.T) {
	nodes := make([]Node, MaxNodes+1)
	for i := range nodes {
		nodes[i] = Node{ID: string(rune('a'+i%26)) + string(rune(i)), Type: NodeTypeOutput}
	}
	def := Definition{Nodes: nodes}
	if err := def.Validate(); err == nil {
		t.Fatal("expected too-many-nodes to be rejected")
	}
}

func TestValidateAcceptsAtLimit(t *testing.T) {
	def := Definition{Nodes: []Node{{ID: "a", Type: NodeTypeOutput}}}
	if err := def.Validate(); err != nil {
		t.Fatalf("expected a single-node definition to validate, got %v", err)
	}
}
