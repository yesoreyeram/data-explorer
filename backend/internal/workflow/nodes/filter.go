package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blues/jsonata-go"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// FilterNode keeps only the rows for which a JSONata boolean expression
// evaluates truthy, e.g. `amount > 1000 and status = "active"`.
type FilterNode struct{}

type FilterConfig struct {
	Expression string `json:"expression"`
}

func (n *FilterNode) Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error) {
	var cfg FilterConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid filter config: %w", err)
	}
	if cfg.Expression == "" {
		return connections.QueryResult{}, fmt.Errorf("filter node requires a jsonata expression")
	}

	expr, err := jsonata.Compile(cfg.Expression)
	if err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid jsonata expression: %w", err)
	}

	input, err := in.SingleInput()
	if err != nil {
		return connections.QueryResult{}, err
	}

	result := connections.QueryResult{Columns: input.Columns, Rows: []map[string]any{}}
	for i, row := range input.Rows {
		out, err := expr.Eval(row)
		if err != nil {
			return connections.QueryResult{}, fmt.Errorf("evaluate jsonata on row %d: %w", i, err)
		}
		if isTruthy(out) {
			result.Rows = append(result.Rows, row)
			result.RowCount++
		}
	}

	return result, nil
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
