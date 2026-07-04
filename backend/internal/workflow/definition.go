// Package workflow implements the pipeline builder: a small DAG of typed
// nodes (source, transform, filter, join, aggregate, output) that is
// authored visually on the frontend (mirroring a Postman/n8n-style canvas)
// and executed server-side, node by node, in topological order.
package workflow

import (
	"encoding/json"
	"fmt"
)

type NodeType string

const (
	NodeTypeSource    NodeType = "source"
	NodeTypeTransform NodeType = "transform"
	NodeTypeFilter    NodeType = "filter"
	NodeTypeJoin      NodeType = "join"
	NodeTypeAggregate NodeType = "aggregate"
	NodeTypeOutput    NodeType = "output"
)

type Node struct {
	ID     string          `json:"id"`
	Type   NodeType        `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
	// Position is UI-only (canvas x/y) but round-tripped so the frontend
	// doesn't lose layout on save.
	Position *Position `json:"position,omitempty"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	// TargetHandle disambiguates multi-input nodes, e.g. join's "left"/"right".
	TargetHandle string `json:"targetHandle,omitempty"`
}

type Definition struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func ParseDefinition(raw json.RawMessage) (Definition, error) {
	var def Definition
	if len(raw) == 0 {
		return def, nil
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return Definition{}, fmt.Errorf("parse workflow definition: %w", err)
	}
	return def, nil
}

// MaxNodes and MaxEdges guardrail how large a single workflow definition may
// be: generous enough for any legitimate pipeline, low enough that a
// pathological definition (crafted or accidental, e.g. a copy-paste loop in
// a client) can't blow up validation/execution time or storage.
const (
	MaxNodes = 200
	MaxEdges = 500
)

// Validate checks structural integrity: unique node ids, edges referencing
// real nodes, known node types, no cycles (the engine requires a DAG), and
// the size guardrails above.
func (d Definition) Validate() error {
	if len(d.Nodes) > MaxNodes {
		return fmt.Errorf("workflow has %d nodes, exceeding the limit of %d", len(d.Nodes), MaxNodes)
	}
	if len(d.Edges) > MaxEdges {
		return fmt.Errorf("workflow has %d edges, exceeding the limit of %d", len(d.Edges), MaxEdges)
	}

	ids := map[string]bool{}
	for _, n := range d.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node missing id")
		}
		if ids[n.ID] {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		ids[n.ID] = true
		switch n.Type {
		case NodeTypeSource, NodeTypeTransform, NodeTypeFilter, NodeTypeJoin, NodeTypeAggregate, NodeTypeOutput:
		default:
			return fmt.Errorf("node %q has unknown type %q", n.ID, n.Type)
		}
	}
	for _, e := range d.Edges {
		if !ids[e.Source] {
			return fmt.Errorf("edge %q references unknown source node %q", e.ID, e.Source)
		}
		if !ids[e.Target] {
			return fmt.Errorf("edge %q references unknown target node %q", e.ID, e.Target)
		}
	}

	if _, err := TopologicalOrder(d); err != nil {
		return err
	}
	return nil
}

// TopologicalOrder returns node IDs in an order where every node appears
// after all of its upstream dependencies, using Kahn's algorithm. Returns an
// error if the graph contains a cycle.
func TopologicalOrder(d Definition) ([]string, error) {
	indegree := map[string]int{}
	adjacency := map[string][]string{}
	for _, n := range d.Nodes {
		indegree[n.ID] = 0
	}
	for _, e := range d.Edges {
		adjacency[e.Source] = append(adjacency[e.Source], e.Target)
		indegree[e.Target]++
	}

	var queue []string
	for _, n := range d.Nodes {
		if indegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, next := range adjacency[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(d.Nodes) {
		return nil, fmt.Errorf("workflow definition contains a cycle")
	}
	return order, nil
}
