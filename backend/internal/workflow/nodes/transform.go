package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blues/jsonata-go"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// TransformNode reshapes each row of its input using a JSONata expression -
// the same query language Postman uses for its post-response scripting, so
// it should feel familiar. The expression is evaluated once per row with the
// row (as a JSON object) as its input context, and must return an object,
// which becomes the row's new shape.
type TransformNode struct{}

type TransformConfig struct {
	Expression string `json:"expression"`
}

func (n *TransformNode) Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error) {
	var cfg TransformConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid transform config: %w", err)
	}
	if cfg.Expression == "" {
		return connections.QueryResult{}, fmt.Errorf("transform node requires a jsonata expression")
	}

	expr, err := jsonata.Compile(cfg.Expression)
	if err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid jsonata expression: %w", err)
	}

	input, err := in.SingleInput()
	if err != nil {
		return connections.QueryResult{}, err
	}

	result := connections.QueryResult{Rows: []map[string]any{}}
	colSeen := map[string]bool{}

	for i, row := range input.Rows {
		out, err := expr.Eval(row)
		if err != nil {
			return connections.QueryResult{}, fmt.Errorf("evaluate jsonata on row %d: %w", i, err)
		}

		newRow, err := toRow(out)
		if err != nil {
			return connections.QueryResult{}, fmt.Errorf("row %d: %w", i, err)
		}

		for k := range newRow {
			if !colSeen[k] {
				colSeen[k] = true
				result.Columns = append(result.Columns, k)
			}
		}
		result.Rows = append(result.Rows, newRow)
		result.RowCount++
	}

	return result, nil
}

// toRow coerces a JSONata evaluation result into a row map. Scalars and
// arrays are wrapped under a single "value" column so the pipeline never
// breaks on a technically-valid-but-non-object expression result.
func toRow(v any) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	// jsonata-go may return map[string]interface{} under a named type in
	// some code paths; re-marshal/unmarshal as a robust fallback.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal transform result: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err == nil {
		return m, nil
	}
	return map[string]any{"value": v}, nil
}
