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
const MaxNodeExecutionDuration = 60 * time.Second

type NodeExecutionResult struct {
	NodeID     string   `json:"nodeId"`
	NodeType   string   `json:"nodeType"`
	NodeName   string   `json:"nodeName"`
	RowsOut    int      `json:"rowsOut"`
	RowCap     int      `json:"rowCap"`
	Truncated  bool     `json:"truncated"`
	DurationMs int64    `json:"durationMs"`
	TimeoutMs  int64    `json:"timeoutMs"`
	Warnings   []string `json:"warnings,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type RunResult struct {
	Output       *dataframe.Frame
	NodeResults  []NodeExecutionResult
	FailedNodeID string
}

type Engine struct {
	registry    *nodes.Registry
	maxRows     int
	nodeTimeout time.Duration
}

func NewEngine(registry *nodes.Registry, maxRows int, nodeTimeout time.Duration) *Engine {
	if maxRows <= 0 {
		maxRows = MaxRowsPerNode
	}
	if nodeTimeout <= 0 {
		nodeTimeout = MaxNodeExecutionDuration
	}
	return &Engine{registry: registry, maxRows: maxRows, nodeTimeout: nodeTimeout}
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
	sourceHints := e.projectionHints(def)
	_ = detectFusionGroups(order, buildDAG(def))

	for _, id := range order {
		node := nodeByID[id]

		execInput := nodes.ExecInput{Config: node.Config, Inputs: map[string]*dataframe.Frame{}}
		if hint, ok := sourceHints[id]; ok && len(hint.Columns) > 0 && node.Type == NodeTypeSource {
			execInput.Projection = &hint
		}
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

		nodeCtx, cancel := context.WithTimeout(ctx, e.nodeTimeout)
		start := time.Now()
		out, execErr := executor.Execute(nodeCtx, deps, execInput)
		cancel()
		duration := time.Since(start)

		nodeResult := NodeExecutionResult{
			NodeID:     id,
			NodeType:   string(node.Type),
			NodeName:   node.Name,
			RowCap:     e.maxRows,
			DurationMs: duration.Milliseconds(),
			TimeoutMs:  e.nodeTimeout.Milliseconds(),
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
		out.LimitRows(e.maxRows)
		if out.Meta.Truncated {
			out.Meta = out.Meta.WithWarning("Row cap reached at %d rows. Add a filter node or narrow the source query to reduce rows.", e.maxRows)
		} else if out.NumRows() >= int(float64(e.maxRows)*0.8) {
			out.Meta = out.Meta.WithWarning("Row count is %d, at least 80%% of the %d row cap. Add a filter node or narrow the source query before the workflow reaches the hard limit.", out.NumRows(), e.maxRows)
		}

		nodeResult.RowsOut = out.NumRows()
		nodeResult.Truncated = out.Meta.Truncated
		nodeResult.Warnings = append([]string(nil), out.Meta.Warnings...)
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
