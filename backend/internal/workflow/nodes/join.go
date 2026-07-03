package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// JoinNode combines two upstream frames (wired to its "left" and "right"
// input handles on the canvas) on equal key values, SQL-JOIN style. The
// actual join algorithm lives in the standalone dataframe package
// (dataframe.Join) - this node is just the config/wiring adapter.
type JoinNode struct{}

type JoinConfig struct {
	LeftKey     string             `json:"leftKey"`
	RightKey    string             `json:"rightKey"`
	Type        dataframe.JoinType `json:"type"`
	RightPrefix string             `json:"rightPrefix"`
}

func (n *JoinNode) Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error) {
	var cfg JoinConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid join config: %w", err)
	}

	left, leftOK := in.Inputs["left"]
	right, rightOK := in.Inputs["right"]
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("join node requires two inputs wired to the \"left\" and \"right\" handles")
	}

	out, err := dataframe.Join(left, right, dataframe.JoinOptions{
		LeftKey:     cfg.LeftKey,
		RightKey:    cfg.RightKey,
		Type:        cfg.Type,
		RightPrefix: cfg.RightPrefix,
	})
	if err != nil {
		return nil, err
	}
	out.Meta.SourceType = "node:join"
	out.Meta.Lineage = []string{left.Meta.Name, right.Meta.Name}
	return out, nil
}
