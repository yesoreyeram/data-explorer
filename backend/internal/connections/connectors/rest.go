package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

type RESTConfig struct {
	BaseURL  string `json:"baseUrl"`
	AuthType string `json:"authType"` // none | basic | bearer | apiKey
	APIKeyHeader string `json:"apiKeyHeader,omitempty"`
}

type REST struct {
	client *http.Client
}

func NewREST() *REST {
	return &REST{client: &http.Client{Timeout: 30 * time.Second}}
}

func (r *REST) buildRequest(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*http.Request, error) {
	var cfg RESTConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid rest config: %w", err)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("baseUrl is required")
	}

	base, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseUrl: %w", err)
	}
	if base.Scheme != "https" && base.Scheme != "http" {
		return nil, fmt.Errorf("baseUrl must be http(s)")
	}

	target, err := base.Parse(strings.TrimPrefix(spec.Path, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if len(spec.Query) > 0 {
		q := target.Query()
		for k, v := range spec.Query {
			q.Set(k, v)
		}
		target.RawQuery = q.Encode()
	}

	method := spec.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if len(spec.Body) > 0 {
		body = bytes.NewReader(spec.Body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	switch cfg.AuthType {
	case "basic":
		req.SetBasicAuth(secret["username"], secret["password"])
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secret["bearerToken"])
	case "apiKey":
		header := cfg.APIKeyHeader
		if header == "" {
			header = "X-Api-Key"
		}
		req.Header.Set(header, secret["apiKey"])
	}

	return req, nil
}

func (r *REST) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := r.buildRequest(ctx, cfgJSON, secret, connections.QuerySpec{Method: http.MethodGet})
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	return nil
}

func (r *REST) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (connections.QueryResult, error) {
	req, err := r.buildRequest(ctx, cfgJSON, secret, spec)
	if err != nil {
		return connections.QueryResult{}, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return connections.QueryResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	const maxBody = 25 * 1024 * 1024 // 25MB response cap
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return connections.QueryResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return connections.QueryResult{}, fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, truncateForError(raw))
	}

	var decoded any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return connections.QueryResult{}, fmt.Errorf("response is not valid JSON: %w", err)
		}
	}

	return toQueryResult(decoded, connections.EffectiveRowLimit(spec.RowLimit)), nil
}

// toQueryResult normalizes an arbitrary JSON payload (object, array of
// objects, or scalar) into the tabular QueryResult shape used everywhere
// else in the pipeline, so downstream nodes (JSONata transform, table view)
// don't need to special-case REST responses.
func toQueryResult(decoded any, limit int) connections.QueryResult {
	result := connections.QueryResult{Rows: []map[string]any{}}

	var items []any
	switch v := decoded.(type) {
	case []any:
		items = v
	case map[string]any:
		items = []any{v}
	default:
		items = []any{map[string]any{"value": v}}
	}

	colSeen := map[string]bool{}
	for _, item := range items {
		if result.RowCount >= limit {
			result.Truncated = true
			break
		}
		obj, ok := item.(map[string]any)
		if !ok {
			obj = map[string]any{"value": item}
		}
		for k := range obj {
			if !colSeen[k] {
				colSeen[k] = true
				result.Columns = append(result.Columns, k)
			}
		}
		result.Rows = append(result.Rows, obj)
		result.RowCount++
	}

	return result
}

func truncateForError(b []byte) string {
	const max = 500
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
