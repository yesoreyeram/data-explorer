package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// SourceNode pulls data into the pipeline from a saved Connection. It never
// sees connection secrets directly - it just names a connection ID and a
// query; connections.Service resolves and decrypts credentials internally.
type SourceNode struct{}

type SourceConfig struct {
	ConnectionID string                `json:"connectionId"`
	Query        connections.QuerySpec `json:"query"`
}

func (n *SourceNode) Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error) {
	var cfg SourceConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid source config: %w", err)
	}
	if cfg.ConnectionID == "" {
		return connections.QueryResult{}, fmt.Errorf("source node requires connectionId")
	}
	if deps.Connections == nil {
		return connections.QueryResult{}, fmt.Errorf("connections service not available")
	}
	return deps.Connections.Query(ctx, cfg.ConnectionID, cfg.Query)
}
