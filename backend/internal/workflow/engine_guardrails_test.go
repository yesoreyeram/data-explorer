package workflow

import (
	"context"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// hugeSource simulates a node whose output would exceed MaxRowsPerNode (e.g.
// a join with a low-selectivity key fanning rows out well past either
// input's size), to verify the engine's per-node guardrail catches it even
// though no connector-level row limit applies to an in-process node.
type hugeSource struct{}

func (hugeSource) Execute(ctx context.Context, deps nodes.Deps, in nodes.ExecInput) (*dataframe.Frame, error) {
	rows := make([]map[string]any, MaxRowsPerNode+500)
	for i := range rows {
		rows[i] = map[string]any{"n": i}
	}
	return dataframe.FromRecords(rows), nil
}

func TestEngineCapsPerNodeRowCount(t *testing.T) {
	registry := nodes.NewRegistry()
	registry.Register("source", hugeSource{})

	def := Definition{Nodes: []Node{{ID: "src", Type: NodeTypeSource}}}

	engine := NewEngine(registry)
	result, err := engine.Run(context.Background(), def, nodes.Deps{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Output.NumRows() != MaxRowsPerNode {
		t.Fatalf("expected output capped at %d rows, got %d", MaxRowsPerNode, result.Output.NumRows())
	}
	if !result.Output.Meta.Truncated {
		t.Fatal("expected Meta.Truncated to be set by the per-node guardrail")
	}
	if len(result.NodeResults) != 1 || !result.NodeResults[0].Truncated || result.NodeResults[0].RowCap != MaxRowsPerNode {
		t.Fatalf("expected node result to include row cap/truncated metadata, got %+v", result.NodeResults)
	}
	if len(result.NodeResults[0].Warnings) == 0 {
		t.Fatal("expected row-cap warning on node result")
	}
}
