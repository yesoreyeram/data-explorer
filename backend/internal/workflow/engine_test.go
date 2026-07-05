package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// stubSource is a test-only node.Executor that returns a fixed dataset
// instead of dialing out to a real connection, so the engine's wiring and
// the JSONata-backed transform/filter nodes can be exercised without a
// database.
type stubSource struct{ result *dataframe.Frame }

func (s stubSource) Execute(ctx context.Context, deps nodes.Deps, in nodes.ExecInput) (*dataframe.Frame, error) {
	return s.result, nil
}

func TestEngineRunEndToEnd(t *testing.T) {
	registry := nodes.NewRegistry()
	registry.Register("source", stubSource{result: dataframe.FromRecords([]map[string]any{
		{"name": "alice", "amount": float64(150)},
		{"name": "bob", "amount": float64(40)},
	})})
	registry.Register("transform", &nodes.TransformNode{})
	registry.Register("filter", &nodes.FilterNode{})
	registry.Register("output", &nodes.OutputNode{})

	def := Definition{
		Nodes: []Node{
			{ID: "src", Type: NodeTypeSource},
			{ID: "flt", Type: NodeTypeFilter, Config: json.RawMessage(`{"expression":"amount > 100"}`)},
			{ID: "xfm", Type: NodeTypeTransform, Config: json.RawMessage(`{"expression":"{\"who\": name, \"big\": amount}"}`)},
			{ID: "out", Type: NodeTypeOutput},
		},
		Edges: []Edge{
			{ID: "e1", Source: "src", Target: "flt"},
			{ID: "e2", Source: "flt", Target: "xfm"},
			{ID: "e3", Source: "xfm", Target: "out"},
		},
	}

	engine := NewEngine(registry, MaxRowsPerNode, 0)
	result, err := engine.Run(context.Background(), def, nodes.Deps{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.NodeResults) != 4 {
		t.Fatalf("expected 4 node results, got %d", len(result.NodeResults))
	}
	if result.Output.NumRows() != 1 {
		t.Fatalf("expected filter to narrow to 1 row, got %d", result.Output.NumRows())
	}
	if result.Output.Rows()[0]["who"] != "alice" {
		t.Fatalf("expected transformed row for alice, got %+v", result.Output.Rows()[0])
	}
}

func TestEngineStopsAtFirstFailingNode(t *testing.T) {
	registry := nodes.NewRegistry()
	registry.Register("source", stubSource{result: dataframe.FromRecords([]map[string]any{{"x": float64(1)}})})
	registry.Register("filter", &nodes.FilterNode{})

	def := Definition{
		Nodes: []Node{
			{ID: "src", Type: NodeTypeSource},
			{ID: "flt", Type: NodeTypeFilter, Config: json.RawMessage(`{"expression":"("}`)}, // invalid jsonata
		},
		Edges: []Edge{{ID: "e1", Source: "src", Target: "flt"}},
	}

	engine := NewEngine(registry, MaxRowsPerNode, 0)
	result, err := engine.Run(context.Background(), def, nodes.Deps{})
	if err == nil {
		t.Fatal("expected engine to surface the node error")
	}
	if result.FailedNodeID != "flt" {
		t.Fatalf("expected failed node to be 'flt', got %q", result.FailedNodeID)
	}
}
