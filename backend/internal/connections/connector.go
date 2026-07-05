package connections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

type PaginationSpec struct {
	Strategy                string `json:"strategy,omitempty"`
	ItemsPath               string `json:"itemsPath,omitempty"`
	OffsetParam             string `json:"offsetParam,omitempty"`
	LimitParam              string `json:"limitParam,omitempty"`
	PageParam               string `json:"pageParam,omitempty"`
	PageSizeParam           string `json:"pageSizeParam,omitempty"`
	CursorParam             string `json:"cursorParam,omitempty"`
	CursorPath              string `json:"cursorPath,omitempty"`
	PageSize                int    `json:"pageSize,omitempty"`
	GraphQLCursorVariable   string `json:"graphqlCursorVariable,omitempty"`
	GraphQLPageSizeVariable string `json:"graphqlPageSizeVariable,omitempty"`
	MaxPages                int    `json:"maxPages,omitempty"`
}

type GraphQLSpec struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName,omitempty"`
	DataPath      string         `json:"dataPath,omitempty"`
}

type CloudQuerySpec struct {
	Query                     string            `json:"query,omitempty"`
	LogGroupNames             []string          `json:"logGroupNames,omitempty"`
	StartTime                 *time.Time        `json:"startTime,omitempty"`
	EndTime                   *time.Time        `json:"endTime,omitempty"`
	TableName                 string            `json:"tableName,omitempty"`
	IndexName                 string            `json:"indexName,omitempty"`
	Scan                      bool              `json:"scan,omitempty"`
	KeyConditionExpression    string            `json:"keyConditionExpression,omitempty"`
	FilterExpression          string            `json:"filterExpression,omitempty"`
	ExpressionAttributeNames  map[string]string `json:"expressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]any    `json:"expressionAttributeValues,omitempty"`
	Bucket                    string            `json:"bucket,omitempty"`
	Key                       string            `json:"key,omitempty"`
	Prefix                    string            `json:"prefix,omitempty"`
	Format                    string            `json:"format,omitempty"`
	Delimiter                 string            `json:"delimiter,omitempty"`
}

type QuerySpec struct {
	SQL            string            `json:"sql,omitempty"`
	Params         []any             `json:"params,omitempty"`
	Method         string            `json:"method,omitempty"`
	Path           string            `json:"path,omitempty"`
	Query          map[string]string `json:"query,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           json.RawMessage   `json:"body,omitempty"`
	RowLimit       int               `json:"rowLimit,omitempty"`
	ProjectionHint []string          `json:"-"`
	Pagination     *PaginationSpec   `json:"pagination,omitempty"`
	GraphQL        *GraphQLSpec      `json:"graphql,omitempty"`
	Cloud          *CloudQuerySpec   `json:"cloud,omitempty"`
}

const DefaultRowLimit = 1_000

var MaxRowLimit = 10_000

func SetMaxRowLimit(maxRows int) {
	if maxRows > 0 {
		MaxRowLimit = maxRows
	}
}

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

type Connector interface {
	Test(ctx context.Context, config json.RawMessage, secret map[string]string) error
	Execute(ctx context.Context, config json.RawMessage, secret map[string]string, spec QuerySpec) (*dataframe.Frame, error)
}

type Registry struct {
	connectors map[string]Connector
}

func NewRegistry() *Registry                              { return &Registry{connectors: map[string]Connector{}} }
func (r *Registry) Register(connType string, c Connector) { r.connectors[connType] = c }
func (r *Registry) Get(connType string) (Connector, error) {
	c, ok := r.connectors[connType]
	if !ok {
		return nil, ErrUnsupportedType
	}
	return c, nil
}
