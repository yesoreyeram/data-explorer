package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

const DefaultInputKey = "default"

type ProjectionHint struct {
	Columns []string
}

type ExecInput struct {
	Inputs     map[string]*dataframe.Frame
	Config     json.RawMessage
	Projection *ProjectionHint
}

func (in ExecInput) SingleInput() (*dataframe.Frame, error) {
	if f, ok := in.Inputs[DefaultInputKey]; ok {
		return f, nil
	}
	for _, f := range in.Inputs {
		return f, nil
	}
	return nil, fmt.Errorf("no input available")
}

type Deps struct{ Connections *connections.Service }

type Executor interface {
	Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error)
}

type Registry struct{ executors map[string]Executor }

func NewRegistry() *Registry                             { return &Registry{executors: map[string]Executor{}} }
func (r *Registry) Register(nodeType string, e Executor) { r.executors[nodeType] = e }
func (r *Registry) Get(nodeType string) (Executor, error) {
	e, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("no executor registered for node type %q", nodeType)
	}
	return e, nil
}

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
