package nodes

import (
	"context"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// OutputNode is a terminal pass-through marker: it exists so a workflow can
// explicitly designate which branch's result is "the" output when a
// pipeline fans out into multiple dead-end branches (e.g. one for a table
// view, one written elsewhere later).
type OutputNode struct{}

func (n *OutputNode) Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error) {
	return in.SingleInput()
}
