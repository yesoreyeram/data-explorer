package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// JoinNode combines two upstream tables (wired to its "left" and "right"
// input handles on the canvas) on equal key values, similar to a SQL JOIN.
type JoinNode struct{}

type JoinType string

const (
	JoinTypeInner JoinType = "inner"
	JoinTypeLeft  JoinType = "left"
)

type JoinConfig struct {
	LeftKey  string   `json:"leftKey"`
	RightKey string   `json:"rightKey"`
	Type     JoinType `json:"type"`
	// Prefix is applied to right-side columns that collide with left-side
	// column names, so joined rows don't silently lose data.
	RightPrefix string `json:"rightPrefix"`
}

func (n *JoinNode) Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error) {
	var cfg JoinConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid join config: %w", err)
	}
	if cfg.LeftKey == "" || cfg.RightKey == "" {
		return connections.QueryResult{}, fmt.Errorf("join node requires leftKey and rightKey")
	}
	if cfg.Type == "" {
		cfg.Type = JoinTypeInner
	}
	if cfg.RightPrefix == "" {
		cfg.RightPrefix = "right_"
	}

	left, leftOK := in.Inputs["left"]
	right, rightOK := in.Inputs["right"]
	if !leftOK || !rightOK {
		return connections.QueryResult{}, fmt.Errorf("join node requires two inputs wired to the \"left\" and \"right\" handles")
	}

	rightByKey := map[any][]map[string]any{}
	for _, row := range right.Rows {
		k := row[cfg.RightKey]
		rightByKey[k] = append(rightByKey[k], row)
	}

	leftCols := map[string]bool{}
	for _, c := range left.Columns {
		leftCols[c] = true
	}

	result := connections.QueryResult{Rows: []map[string]any{}}
	colSeen := map[string]bool{}
	addCol := func(c string) {
		if !colSeen[c] {
			colSeen[c] = true
			result.Columns = append(result.Columns, c)
		}
	}

	for _, lrow := range left.Rows {
		matches := rightByKey[lrow[cfg.LeftKey]]
		if len(matches) == 0 && cfg.Type == JoinTypeLeft {
			merged := cloneRow(lrow)
			for c := range merged {
				addCol(c)
			}
			result.Rows = append(result.Rows, merged)
			result.RowCount++
			continue
		}
		for _, rrow := range matches {
			merged := cloneRow(lrow)
			for k, v := range rrow {
				outKey := k
				if leftCols[k] {
					outKey = cfg.RightPrefix + k
				}
				merged[outKey] = v
			}
			for c := range merged {
				addCol(c)
			}
			result.Rows = append(result.Rows, merged)
			result.RowCount++
		}
	}

	return result, nil
}

func cloneRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}
