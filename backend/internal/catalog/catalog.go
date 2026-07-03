// Package catalog is a small, first-party, static registry of well-known API
// integrations - purely a prefill convenience for the existing rest/graphql
// connection form. It reuses domain.ConnectionType and the AuthType/AuthConfig
// vocabulary connectors/httpauth.go already speaks, so a chosen entry maps
// directly onto fields the connection form already knows how to render.
//
// This is deliberately not a live client of any external registry: the data
// is authored once, in seed.go, and searched entirely in memory. There is
// nothing to cache and nothing to call over the network.
package catalog

import "github.com/yesoreyeram/data-explorer/backend/internal/domain"

// Entry describes one well-known integration. AuthConfig carries only
// non-secret fields (e.g. oauth2TokenUrl, apiKeyHeader) matching
// connectors/httpauth.go's AuthConfig - never a credential.
type Entry struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Category    string                `json:"category"`
	Type        domain.ConnectionType `json:"type"` // "rest" | "graphql"
	BaseURL     string                `json:"baseUrl,omitempty"`
	Endpoint    string                `json:"endpoint,omitempty"`
	AuthType    string                `json:"authType"`
	AuthConfig  map[string]any        `json:"authConfig,omitempty"`
	DocsURL     string                `json:"docsUrl,omitempty"`
}
