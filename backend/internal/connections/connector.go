// Package connections manages reusable, credentialed links to external data
// sources (databases, REST APIs, ...). It owns the only code path in the
// system allowed to decrypt connection secrets, and it does so strictly
// in-memory, for the duration of a single dial-out.
package connections

import (
	"context"
	"encoding/json"
	"errors"
)

// QuerySpec is a connector-agnostic request. Which fields apply depends on
// the connection type: SQL connectors read SQL/Params, the REST connector
// reads Method/Path/Query/Body.
type QuerySpec struct {
	SQL      string            `json:"sql,omitempty"`
	Params   []any             `json:"params,omitempty"`
	Method   string            `json:"method,omitempty"`
	Path     string            `json:"path,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     json.RawMessage   `json:"body,omitempty"`
	RowLimit int               `json:"rowLimit,omitempty"`
}

type QueryResult struct {
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"rowCount"`
	Truncated bool            `json:"truncated"`
}

// MaxRowLimit is a hard ceiling applied regardless of what a caller requests,
// so a runaway exploration query cannot exhaust server memory or leak an
// unbounded amount of data through the API.
const MaxRowLimit = 10_000

// DefaultRowLimit is used when a caller does not specify one.
const DefaultRowLimit = 1_000

func EffectiveRowLimit(requested int) int {
	if requested <= 0 {
		return DefaultRowLimit
	}
	if requested > MaxRowLimit {
		return MaxRowLimit
	}
	return requested
}

var ErrUnsupportedType = errors.New("unsupported connection type")

// Connector is implemented once per external system type (Postgres, MySQL,
// REST, ...). Secrets are passed in decrypted, in-memory, and must never be
// logged or echoed back in results/errors.
type Connector interface {
	// Test verifies connectivity/credentials without returning data.
	Test(ctx context.Context, config json.RawMessage, secret map[string]string) error
	// Execute runs a read query against the source and returns a bounded result set.
	Execute(ctx context.Context, config json.RawMessage, secret map[string]string, spec QuerySpec) (QueryResult, error)
}

type Registry struct {
	connectors map[string]Connector
}

func NewRegistry() *Registry {
	return &Registry{connectors: map[string]Connector{}}
}

func (r *Registry) Register(connType string, c Connector) {
	r.connectors[connType] = c
}

func (r *Registry) Get(connType string) (Connector, error) {
	c, ok := r.connectors[connType]
	if !ok {
		return nil, ErrUnsupportedType
	}
	return c, nil
}
