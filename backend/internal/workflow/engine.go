package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// MaxRowsPerNode bounds the output of any single node, regardless of type -
// see the guardrail comment in Run for why this matters most for join.
const MaxRowsPerNode = 100_000

type NodeExecutionResult struct {
	NodeID     string `json:"nodeId"`
	NodeType   string `json:"nodeType"`
	NodeName   string `json:"nodeName"`
	RowsOut    int    `json:"rowsOut"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type RunResult struct {
	Output       *dataframe.Frame
	NodeResults  []NodeExecutionResult
	FailedNodeID string
}

type Engine struct {
	registry *nodes.Registry
}

func NewEngine(registry *nodes.Registry) *Engine {
	return &Engine{registry: registry}
}

// Run executes every node in the definition in dependency order, wiring each
// node's declared inputs from its upstream nodes' outputs. Execution stops
// at the first node that errors; everything up to that point is still
// reported in RunResult.NodeResults so the caller can show partial progress.
func (e *Engine) Run(ctx context.Context, def Definition, deps nodes.Deps) (RunResult, error) {
	order, err := TopologicalOrder(def)
	if err != nil {
		return RunResult{}, err
	}

	nodeByID := make(map[string]Node, len(def.Nodes))
	for _, n := range def.Nodes {
		nodeByID[n.ID] = n
	}

	incoming := map[string][]Edge{}
	for _, edge := range def.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge)
	}

	outputs := map[string]*dataframe.Frame{}
	result := RunResult{}

	for _, id := range order {
		node := nodeByID[id]

		execInput := nodes.ExecInput{Config: node.Config, Inputs: map[string]*dataframe.Frame{}}
		edgesIn := incoming[id]
		for _, edge := range edgesIn {
			upstream, ok := outputs[edge.Source]
			if !ok {
				return result, fmt.Errorf("internal error: node %q executed before its dependency %q", id, edge.Source)
			}
			handle := edge.TargetHandle
			if handle == "" {
				handle = nodes.DefaultInputKey
			}
			execInput.Inputs[handle] = upstream
			execInput.Inputs[edge.Source] = upstream
		}

		executor, err := e.registry.Get(string(node.Type))
		if err != nil {
			return result, fmt.Errorf("node %q: %w", id, err)
		}

		start := time.Now()
		out, execErr := executor.Execute(ctx, deps, execInput)
		duration := time.Since(start)

		nodeResult := NodeExecutionResult{
			NodeID:     id,
			NodeType:   string(node.Type),
			NodeName:   node.Name,
			DurationMs: duration.Milliseconds(),
		}
		if execErr != nil {
			nodeResult.Error = execErr.Error()
			result.NodeResults = append(result.NodeResults, nodeResult)
			result.FailedNodeID = id
			return result, fmt.Errorf("node %q (%s) failed: %w", id, node.Type, execErr)
		}

		if out.Meta.Name == "" {
			out.Meta.Name = node.Name
		}
		out.Meta.SourceID = id

		// Defense in depth: a source node's output is already bounded by the
		// connection's row limit, but a join can fan rows out well past
		// either input's size (a cartesian-ish blow-up on a low-selectivity
		// key), and that growth happens entirely in-process with no
		// connector-level guardrail to catch it. Capping every node's output
		// here protects the rest of the pipeline regardless of node type.
		out.LimitRows(MaxRowsPerNode)

		nodeResult.RowsOut = out.NumRows()
		result.NodeResults = append(result.NodeResults, nodeResult)
		outputs[id] = out
		result.Output = out
	}

	// Prefer the last explicit "output" node's result, if any, over the
	// last-executed node overall (which may just be an intermediate branch).
	for i := len(order) - 1; i >= 0; i-- {
		if nodeByID[order[i]].Type == NodeTypeOutput {
			result.Output = outputs[order[i]]
			break
		}
	}

	return result, nil
}
