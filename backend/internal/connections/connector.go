// Package connections manages reusable, credentialed links to external data
// sources (databases, REST/GraphQL APIs, ...). It owns the only code path in
// the system allowed to decrypt connection secrets, and it does so strictly
// in-memory, for the duration of a single dial-out. Every connector returns
// a dataframe.Frame - the same typed, metadata-rich tabular contract used
// throughout the workflow engine - so a source node's output composes with
// every other node regardless of which system produced it.
package connections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// PaginationSpec configures how a connector should walk a multi-page result
// set. Which fields apply depends on Strategy; see
// backend/pkg/httpclient/pagination*.go for the underlying implementations.
type PaginationSpec struct {
	// Strategy: "" or "none" (single request), "offset", "page", "cursor",
	// "linkHeader", or "graphqlRelay".
	Strategy string `json:"strategy,omitempty"`

	// ItemsPath locates the array of rows within each page's JSON body for
	// the offset/page/cursor/linkHeader strategies, e.g. "data.items".
	// Empty means the response body itself is the array.
	ItemsPath string `json:"itemsPath,omitempty"`

	OffsetParam string `json:"offsetParam,omitempty"`
	LimitParam  string `json:"limitParam,omitempty"`

	PageParam     string `json:"pageParam,omitempty"`
	PageSizeParam string `json:"pageSizeParam,omitempty"`

	CursorParam string `json:"cursorParam,omitempty"`
	CursorPath  string `json:"cursorPath,omitempty"`

	PageSize int `json:"pageSize,omitempty"`

	// GraphQLDataPath/CursorVariable/PageSizeVariable configure the
	// "graphqlRelay" strategy - see GraphQLSpec for the query itself.
	GraphQLCursorVariable   string `json:"graphqlCursorVariable,omitempty"`
	GraphQLPageSizeVariable string `json:"graphqlPageSizeVariable,omitempty"`

	// MaxPages guardrails how many pages are fetched; see
	// httpclient.DefaultMaxPages/HardMaxPages for the effective bounds.
	MaxPages int `json:"maxPages,omitempty"`
}

// GraphQLSpec is the query payload for a "graphql" connection.
type GraphQLSpec struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName,omitempty"`
	// DataPath locates the connection field to read rows from, e.g.
	// "data.search" (relay edges/node convention) or "data.items" (a plain
	// array). See connectors/graphql.go for how each shape is unwrapped.
	DataPath string `json:"dataPath,omitempty"`
}

// CloudQuerySpec is the query payload for the "aws", "gcp", and "azure"
// connection types. Which fields apply depends on the connection's
// configured Service (see each cloud's connector file for the exact
// mapping) - e.g. Athena/BigQuery/Log Analytics read Query, DynamoDB reads
// TableName+KeyConditionExpression, S3/GCS/Blob Storage read
// Bucket+Key/Prefix.
type CloudQuerySpec struct {
	// Query is a SQL (Athena, BigQuery) or KQL (Azure Log Analytics) query,
	// or a CloudWatch Logs Insights query string.
	Query string `json:"query,omitempty"`

	// LogGroupNames (CloudWatch Logs) and TimeRange (CloudWatch Logs, Log
	// Analytics) bound a log query.
	LogGroupNames []string   `json:"logGroupNames,omitempty"`
	StartTime     *time.Time `json:"startTime,omitempty"`
	EndTime       *time.Time `json:"endTime,omitempty"`

	// DynamoDB
	TableName                 string            `json:"tableName,omitempty"`
	IndexName                 string            `json:"indexName,omitempty"`
	Scan                      bool              `json:"scan,omitempty"` // true = Scan, false = Query
	KeyConditionExpression    string            `json:"keyConditionExpression,omitempty"`
	FilterExpression          string            `json:"filterExpression,omitempty"`
	ExpressionAttributeNames  map[string]string `json:"expressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]any    `json:"expressionAttributeValues,omitempty"`

	// Object storage (S3 / GCS / Azure Blob Storage): Key reads one object;
	// Prefix (with Key empty) lists objects under it as rows instead.
	Bucket    string `json:"bucket,omitempty"` // container name, for Azure
	Key       string `json:"key,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Format    string `json:"format,omitempty"` // csv|json|ndjson, default inferred from the key's extension
	Delimiter string `json:"delimiter,omitempty"`
}

// QuerySpec is a connector-agnostic request. Which fields apply depends on
// the connection type: SQL connectors read SQL/Params, REST reads
// Method/Path/Query/Body(+Pagination), GraphQL reads GraphQL(+Pagination),
// and the cloud provider connectors (aws/gcp/azure) read Cloud.
type QuerySpec struct {
	SQL      string            `json:"sql,omitempty"`
	Params   []any             `json:"params,omitempty"`
	Method   string            `json:"method,omitempty"`
	Path     string            `json:"path,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     json.RawMessage   `json:"body,omitempty"`
	RowLimit int               `json:"rowLimit,omitempty"`

	Pagination *PaginationSpec `json:"pagination,omitempty"`
	GraphQL    *GraphQLSpec    `json:"graphql,omitempty"`
	Cloud      *CloudQuerySpec `json:"cloud,omitempty"`
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
// REST, GraphQL, ...). Secrets are passed in decrypted, in-memory, and must
// never be logged or echoed back in results/errors. Execute's returned
// Frame should leave Meta.SourceType/SourceID/Name unset - Service fills
// those in from the Connection record, since the connector itself doesn't
// know its own connection's ID or display name.
type Connector interface {
	// Test verifies connectivity/credentials without returning data.
	Test(ctx context.Context, config json.RawMessage, secret map[string]string) error
	// Execute runs a read query against the source and returns a bounded,
	// typed result set.
	Execute(ctx context.Context, config json.RawMessage, secret map[string]string, spec QuerySpec) (*dataframe.Frame, error)
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
