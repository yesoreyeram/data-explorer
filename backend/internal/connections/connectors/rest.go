package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/safejson"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

type RESTConfig struct {
	BaseURL string `json:"baseUrl"`
	AuthConfig
}

type REST struct{ opts Options }

func NewREST(opts Options) *REST { return &REST{opts: opts} }

func (r *REST) parseConfig(cfgJSON json.RawMessage) (RESTConfig, error) {
	var cfg RESTConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return RESTConfig{}, connections.NewConfigError("REST configuration is not valid JSON.")
	}
	if cfg.BaseURL == "" {
		return RESTConfig{}, connections.NewConfigError("Base URL is required.")
	}
	if strings.ContainsAny(cfg.BaseURL, "{}") {
		return RESTConfig{}, connections.NewConfigError("Replace placeholder values in the base URL before saving.")
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return RESTConfig{}, connections.NewConfigError("Base URL must be a valid HTTP or HTTPS URL.")
	}
	return cfg, nil
}

func (r *REST) client(ctx context.Context, cfg RESTConfig, secret map[string]string) (*httpclient.Client, error) {
	dial := r.opts.dial(ctx)
	auth, err := buildAuthenticator(ctx, cfg.AuthConfig, secret, dial)
	if err != nil {
		return nil, fmt.Errorf("configure authentication: %w", err)
	}
	return httpclient.New(httpclient.Config{
		Timeout:          r.opts.Guardrails.NodeTimeout,
		MaxResponseBytes: r.opts.Guardrails.MaxBodyBytes,
		MaxRedirects:     r.opts.Guardrails.MaxRedirects,
		DecompressRatio:  r.opts.Guardrails.DecompressRatio,
		Auth:             auth,
		Retry:            httpclient.DefaultRetryPolicy,
		DialContext:      dial,
		UserAgent:        r.opts.UserAgent,
	}), nil
}

func (r *REST) buildRequest(ctx context.Context, cfg RESTConfig, spec connections.QuerySpec) (*http.Request, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, err
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
		body = strings.NewReader(string(spec.Body))
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if len(spec.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range spec.Headers {
		if r.opts.StrictHeaders {
			if err := validateOutboundHeader(k); err != nil {
				return nil, err
			}
		}
		req.Header.Set(k, v)
	}
	return req, nil
}

// reservedHeaders are hop-by-hop or transport-controlled headers a caller must
// not be able to set on an outbound request built by the relay.
var reservedHeaders = map[string]struct{}{
	"host":              {},
	"connection":        {},
	"content-length":    {},
	"transfer-encoding": {},
	"keep-alive":        {},
	"proxy-connection":  {},
	"upgrade":           {},
	"te":                {},
	"trailer":           {},
}

func validateOutboundHeader(name string) error {
	if name == "" {
		return connections.NewConfigError("Header name must not be empty.")
	}
	// RFC 7230 token: no separators or control characters.
	for _, c := range name {
		if c <= ' ' || c >= 0x7f || strings.ContainsRune("()<>@,;:\\\"/[]?={}", c) {
			return connections.NewConfigError(fmt.Sprintf("Header name %q contains invalid characters.", name))
		}
	}
	lower := strings.ToLower(name)
	if _, bad := reservedHeaders[lower]; bad || strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "proxy-") {
		return connections.NewConfigError(fmt.Sprintf("Header %q may not be set.", name))
	}
	return nil
}

func (r *REST) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	cfg, err := r.parseConfig(cfgJSON)
	if err != nil {
		return err
	}
	client, err := r.client(ctx, cfg, secret)
	if err != nil {
		return err
	}
	req, err := r.buildRequest(ctx, cfg, connections.QuerySpec{Method: http.MethodGet})
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

func (r *REST) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	start := time.Now()
	cfg, err := r.parseConfig(cfgJSON)
	if err != nil {
		return nil, err
	}
	client, err := r.client(ctx, cfg, secret)
	if err != nil {
		return nil, err
	}
	req, err := r.buildRequest(ctx, cfg, spec)
	if err != nil {
		return nil, err
	}
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false
	warnings := []string(nil)
	appendPage := func(decoded any, itemsPath string) {
		items := extractItems(decoded, itemsPath)
		for _, item := range items {
			if frame.NumRows() >= limit {
				truncated = true
				return
			}
			frame.AppendRow(toRowMap(item))
		}
	}
	decodeBody := func(body []byte) (any, error) {
		var decoded any
		if len(body) == 0 {
			return nil, nil
		}
		if err := safejson.Unmarshal(body, &decoded, r.opts.Guardrails.JSONMaxDepth, r.opts.Guardrails.JSONMaxElements); err != nil {
			return nil, fmt.Errorf("response is not valid bounded JSON: %w", err)
		}
		return decoded, nil
	}
	if spec.Pagination != nil && spec.Pagination.Strategy != "" && spec.Pagination.Strategy != "none" {
		paginator, err := buildRESTPaginator(spec.Pagination)
		if err != nil {
			return nil, err
		}
		result, err := client.DoPaginated(ctx, req, paginator, maxPagesOf(spec.Pagination, r.opts.Guardrails.MaxPages))
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
				return nil, connections.NewGuardrailError(connections.ErrCodeInvalidConfig, "HTTP response body bytes", r.opts.Guardrails.MaxBodyBytes, int64(len(page.Response.Body))+1, "Reduce page size, add filters, or narrow the request.")
			}
			warnings = appendBodyCapWarning(warnings, len(page.Response.Body))
			decoded, err := decodeBody(page.Response.Body)
			if err != nil {
				return nil, err
			}
			appendPage(decoded, spec.Pagination.ItemsPath)
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
			return nil, connections.NewGuardrailError(connections.ErrCodeInvalidConfig, "HTTP response body bytes", r.opts.Guardrails.MaxBodyBytes, int64(len(resp.Body))+1, "Reduce page size, add filters, or narrow the request.")
		}
		warnings = appendBodyCapWarning(warnings, len(resp.Body))
		decoded, err := decodeBody(resp.Body)
		if err != nil {
			return nil, err
		}
		appendPage(decoded, "")
	}
	frame.LimitColumns(r.opts.Guardrails.MaxColumns)
	frame.SetMeta(dataframe.Metadata{SourceType: "rest", GeneratedAt: start, DurationMs: time.Since(start).Milliseconds(), Truncated: truncated || frame.Meta.Truncated, Warnings: warnings})
	return frame, nil
}

func appendBodyCapWarning(warnings []string, bytesRead int) []string {
	const softRatio = 0.8
	threshold := int(float64(httpclient.DefaultMaxResponseBytes) * softRatio)
	if bytesRead < threshold {
		return warnings
	}
	return append(warnings, fmt.Sprintf("Response body is %d bytes, at least 80%% of the %d byte cap. Narrow the request before it reaches the hard limit.", bytesRead, httpclient.DefaultMaxResponseBytes))
}

func truncateForError(b []byte) string {
	const max = 500
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
