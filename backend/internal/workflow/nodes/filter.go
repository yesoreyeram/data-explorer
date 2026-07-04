package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blues/jsonata-go"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// FilterNode keeps only the rows for which a JSONata boolean expression
// evaluates truthy, e.g. `amount > 1000 and status = "active"`.
type FilterNode struct{}

type FilterConfig struct {
	Expression string `json:"expression"`
}

func (n *FilterNode) Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error) {
	var cfg FilterConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid filter config: %w", err)
	}
	if cfg.Expression == "" {
		return nil, fmt.Errorf("filter node requires a jsonata expression")
	}

	expr, err := jsonata.Compile(cfg.Expression)
	if err != nil {
		return nil, fmt.Errorf("invalid jsonata expression: %w", err)
	}

	input, err := in.SingleInput()
	if err != nil {
		return nil, err
	}

	var evalErr error
	out := input.Filter(func(i int, row map[string]any) bool {
		if evalErr != nil {
			return false
		}
		result, err := expr.Eval(row)
		if err != nil {
			evalErr = fmt.Errorf("evaluate jsonata on row %d: %w", i, err)
			return false
		}
		return isTruthy(result)
	})
	if evalErr != nil {
		return nil, evalErr
	}

	out.Meta.SourceType = "node:filter"
	out.Meta.Lineage = []string{input.Meta.Name}
	return out, nil
}

func isTruthy(v any) bool {
	switch val := v.(type) {
	case nil:
		return false
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case []any:
		return len(val) > 0
	default:
		return true
	}
}
