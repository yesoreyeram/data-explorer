package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// AggregateNode groups rows by a set of key columns and computes summary
// statistics per group, SQL GROUP BY style. The grouping/aggregation logic
// lives in the standalone dataframe package (Frame.GroupBy) - this node is
// just the config/wiring adapter.
type AggregateNode struct{}

type AggregateConfig struct {
	GroupBy      []string        `json:"groupBy"`
	Aggregations []dataframe.Agg `json:"aggregations"`
}

func (n *AggregateNode) Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error) {
	var cfg AggregateConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid aggregate config: %w", err)
	}
	if len(cfg.Aggregations) == 0 {
		return nil, fmt.Errorf("aggregate node requires at least one aggregation")
	}

	input, err := in.SingleInput()
	if err != nil {
		return nil, err
	}

	out, err := input.GroupBy(cfg.GroupBy, cfg.Aggregations)
	if err != nil {
		return nil, err
	}
	out.Meta.SourceType = "node:aggregate"
	out.Meta.Lineage = []string{input.Meta.Name}
	return out, nil
}
