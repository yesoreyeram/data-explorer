// Package nodes implements the executor for each workflow node type. Every
// executor speaks the same tabular contract (connections.QueryResult in,
// connections.QueryResult out) so nodes can be freely rewired on the canvas.
package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// DefaultInputKey is used for single-input nodes; the engine always makes
// the sole upstream result available under this key in addition to the
// producing node's own ID.
const DefaultInputKey = "default"

type ExecInput struct {
	// Inputs is keyed by target handle (e.g. "left"/"right" for join) and,
	// for convenience, also by the producing node's ID and - when there is
	// exactly one upstream node - by DefaultInputKey.
	Inputs map[string]connections.QueryResult
	Config json.RawMessage
}

func (in ExecInput) SingleInput() (connections.QueryResult, error) {
	if r, ok := in.Inputs[DefaultInputKey]; ok {
		return r, nil
	}
	for _, r := range in.Inputs {
		return r, nil
	}
	return connections.QueryResult{}, fmt.Errorf("no input available")
}

// Deps carries services node executors may need to reach out to.
type Deps struct {
	Connections *connections.Service
}

type Executor interface {
	Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error)
}

type Registry struct {
	executors map[string]Executor
}

func NewRegistry() *Registry {
	return &Registry{executors: map[string]Executor{}}
}

func (r *Registry) Register(nodeType string, e Executor) {
	r.executors[nodeType] = e
}

func (r *Registry) Get(nodeType string) (Executor, error) {
	e, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("no executor registered for node type %q", nodeType)
	}
	return e, nil
}

// DefaultRegistry wires up every built-in node type.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("source", &SourceNode{})
	r.Register("transform", &TransformNode{})
	r.Register("filter", &FilterNode{})
	r.Register("join", &JoinNode{})
	r.Register("aggregate", &AggregateNode{})
	r.Register("output", &OutputNode{})
	return r
}
