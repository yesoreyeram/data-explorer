package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/safejson"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

type GraphQLConfig struct {
	Endpoint string `json:"endpoint"`
	AuthConfig
}

type GraphQL struct{ opts Options }

func NewGraphQL(opts Options) *GraphQL { return &GraphQL{opts: opts} }

func (g *GraphQL) parseConfig(cfgJSON json.RawMessage) (GraphQLConfig, error) {
	var cfg GraphQLConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return GraphQLConfig{}, connections.NewConfigError("GraphQL configuration is not valid JSON.")
	}
	if cfg.Endpoint == "" {
		return GraphQLConfig{}, connections.NewConfigError("Endpoint is required.")
	}
	if strings.ContainsAny(cfg.Endpoint, "{}") {
		return GraphQLConfig{}, connections.NewConfigError("Replace placeholder values in the endpoint before saving.")
	}
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return GraphQLConfig{}, connections.NewConfigError("Endpoint must be a valid HTTP or HTTPS URL.")
	}
	return cfg, nil
}

func (g *GraphQL) client(ctx context.Context, cfg GraphQLConfig, secret map[string]string) (*httpclient.Client, error) {
	dial := g.opts.dial(ctx)
	auth, err := buildAuthenticator(ctx, cfg.AuthConfig, secret, dial)
	if err != nil {
		return nil, fmt.Errorf("configure authentication: %w", err)
	}
	return httpclient.New(httpclient.Config{
		Timeout:          g.opts.Guardrails.NodeTimeout,
		MaxResponseBytes: g.opts.Guardrails.MaxBodyBytes,
		MaxRedirects:     g.opts.Guardrails.MaxRedirects,
		DecompressRatio:  g.opts.Guardrails.DecompressRatio,
		Auth:             auth,
		Retry:            httpclient.DefaultRetryPolicy,
		DialContext:      dial,
		UserAgent:        g.opts.UserAgent,
	}), nil
}

func (g *GraphQL) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	cfg, err := g.parseConfig(cfgJSON)
	if err != nil {
		return err
	}
	client, err := g.client(ctx, cfg, secret)
	if err != nil {
		return err
	}
	req, err := httpclient.NewGraphQLRequest(ctx, cfg.Endpoint, httpclient.GraphQLRequest{Query: "{ __typename }"})
	if err != nil {
		return err
	}
	resp, err := client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	return nil
}

func (g *GraphQL) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	start := time.Now()
	if spec.GraphQL == nil || spec.GraphQL.Query == "" {
		return nil, fmt.Errorf("graphql query is required")
	}
	cfg, err := g.parseConfig(cfgJSON)
	if err != nil {
		return nil, err
	}
	client, err := g.client(ctx, cfg, secret)
	if err != nil {
		return nil, err
	}
	req, err := httpclient.NewGraphQLRequest(ctx, cfg.Endpoint, httpclient.GraphQLRequest{Query: spec.GraphQL.Query, Variables: spec.GraphQL.Variables, OperationName: spec.GraphQL.OperationName})
	if err != nil {
		return nil, err
	}
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false
	var warnings []string
	appendPage := func(decoded any) error {
		if errs := httpclient.ParseGraphQLErrors(decoded); len(errs) > 0 {
			return fmt.Errorf("graphql error: %s", errs.Error())
		}
		rows := extractGraphQLRows(decoded, spec.GraphQL.DataPath)
		for _, row := range rows {
			if frame.NumRows() >= limit {
				truncated = true
				return nil
			}
			frame.AppendRow(row)
		}
		return nil
	}
	decodeBody := func(body []byte) (any, error) {
		var decoded any
		if err := safejson.Unmarshal(body, &decoded, g.opts.Guardrails.JSONMaxDepth, g.opts.Guardrails.JSONMaxElements); err != nil {
			return nil, fmt.Errorf("response is not valid bounded JSON: %w", err)
		}
		return decoded, nil
	}
	if spec.Pagination != nil && spec.Pagination.Strategy != "" && spec.Pagination.Strategy != "none" {
		paginator, err := buildGraphQLPaginator(spec.GraphQL.DataPath, spec.Pagination)
		if err != nil {
			return nil, err
		}
		result, err := client.DoPaginated(ctx, req, paginator, maxPagesOf(spec.Pagination, g.opts.Guardrails.MaxPages))
		if err != nil {
			return nil, fmt.Errorf("paginated request failed: %w", err)
		}
		if result.Truncated {
			warnings = append(warnings, fmt.Sprintf("pagination stopped after the %d-page limit; more pages may exist", result.PageCount))
		}
		for _, page := range result.Pages {
			if page.Response.IsError() {
				return nil, fmt.Errorf("upstream returned status %d: %s", page.Response.StatusCode, truncateForError(page.Response.Body))
			}
			if page.Response.Truncated {
				return nil, connections.NewGuardrailError(connections.ErrCodeInvalidConfig, "HTTP response body bytes", g.opts.Guardrails.MaxBodyBytes, int64(len(page.Response.Body))+1, "Reduce page size, add filters, or narrow the request.")
			}
			warnings = appendBodyCapWarning(warnings, len(page.Response.Body))
			decoded, err := decodeBody(page.Response.Body)
			if err != nil {
				return nil, err
			}
			if err := appendPage(decoded); err != nil {
				return nil, err
			}
			if truncated {
				break
			}
		}
	} else {
		resp, err := client.Do(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		if resp.IsError() {
			return nil, fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, truncateForError(resp.Body))
		}
		if resp.Truncated {
			return nil, connections.NewGuardrailError(connections.ErrCodeInvalidConfig, "HTTP response body bytes", g.opts.Guardrails.MaxBodyBytes, int64(len(resp.Body))+1, "Reduce page size, add filters, or narrow the request.")
		}
		warnings = appendBodyCapWarning(warnings, len(resp.Body))
		decoded, err := decodeBody(resp.Body)
		if err != nil {
			return nil, err
		}
		if err := appendPage(decoded); err != nil {
			return nil, err
		}
	}
	frame.LimitColumns(g.opts.Guardrails.MaxColumns)
	frame.SetMeta(dataframe.Metadata{SourceType: "graphql", GeneratedAt: start, DurationMs: time.Since(start).Milliseconds(), Truncated: truncated || frame.Meta.Truncated, Warnings: warnings})
	return frame, nil
}

func extractGraphQLRows(decoded any, dataPath string) []map[string]any {
	node, ok := httpclient.JSONPath(decoded, dataPath)
	if !ok || node == nil {
		return nil
	}
	if arr, ok := node.([]any); ok {
		rows := make([]map[string]any, 0, len(arr))
		for _, item := range arr {
			rows = append(rows, toRowMap(item))
		}
		return rows
	}
	obj, ok := node.(map[string]any)
	if !ok {
		return []map[string]any{toRowMap(node)}
	}
	if edges, ok := obj["edges"].([]any); ok {
		rows := make([]map[string]any, 0, len(edges))
		for _, edge := range edges {
			edgeObj, ok := edge.(map[string]any)
			if !ok {
				rows = append(rows, toRowMap(edge))
				continue
			}
			if node, ok := edgeObj["node"]; ok {
				rows = append(rows, toRowMap(node))
			} else {
				rows = append(rows, edgeObj)
			}
		}
		return rows
	}
	return []map[string]any{obj}
}
