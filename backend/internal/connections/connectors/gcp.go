// Package connectors: GCP support. Mirrors aws.go's shape - one connection
// type ("gcp") covering multiple services, selected by GCPConfig.Service,
// with authentication centralized here so each service file only
// implements its own query semantics.
package connectors

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/option"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// GCPConfig is the non-secret configuration for a "gcp" connection.
// Credentials (secret key "serviceAccountKeyJson") are optional: when
// omitted, the connector falls back to Application Default Credentials -
// the ambient identity of the environment this server runs in (a GCE/GKE
// Workload Identity-bound service account, most commonly), so an operator
// running inside GCP never has to store a long-lived key here at all.
type GCPConfig struct {
	ProjectID string `json:"projectId"`
	// Service selects which GCP service this connection queries:
	// "bigquery" | "gcs".
	Service string `json:"service"`
}

type GCP struct{}

func NewGCP() *GCP { return &GCP{} }

func (g *GCP) parseConfig(cfgJSON json.RawMessage) (GCPConfig, error) {
	var cfg GCPConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return GCPConfig{}, fmt.Errorf("invalid gcp config: %w", err)
	}
	if cfg.ProjectID == "" {
		return GCPConfig{}, fmt.Errorf("projectId is required")
	}
	switch cfg.Service {
	case "bigquery", "gcs":
	default:
		return GCPConfig{}, fmt.Errorf("unsupported gcp service %q", cfg.Service)
	}
	return cfg, nil
}

// gcpClientOptions returns the option.ClientOption needed to authenticate,
// given an optional service account key from the connection's secret.
func gcpClientOptions(secret map[string]string) []option.ClientOption {
	if key := secret["serviceAccountKeyJson"]; key != "" {
		return []option.ClientOption{option.WithCredentialsJSON([]byte(key))}
	}
	return nil
}

func (g *GCP) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	cfg, err := g.parseConfig(cfgJSON)
	if err != nil {
		return err
	}
	opts := gcpClientOptions(secret)

	switch cfg.Service {
	case "bigquery":
		return testBigQuery(ctx, cfg, opts)
	case "gcs":
		return testGCS(ctx, opts)
	default:
		return connections.ErrUnsupportedType
	}
}

func (g *GCP) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	cfg, err := g.parseConfig(cfgJSON)
	if err != nil {
		return nil, err
	}
	if spec.Cloud == nil {
		return nil, fmt.Errorf("this connection requires a cloud query spec")
	}
	opts := gcpClientOptions(secret)

	switch cfg.Service {
	case "bigquery":
		return executeBigQuery(ctx, cfg, opts, spec)
	case "gcs":
		return executeGCS(ctx, opts, spec)
	default:
		return nil, connections.ErrUnsupportedType
	}
}
