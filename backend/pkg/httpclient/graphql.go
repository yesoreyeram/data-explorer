package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GraphQLRequest is the standard `{query, variables, operationName}` POST
// body every GraphQL server accepts.
type GraphQLRequest struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName,omitempty"`
}

// NewGraphQLRequest builds the POST request for a GraphQL operation against
// endpoint. Combine with Client.Do for a single call, or with
// GraphQLRelayPaginator + Client.DoPaginated to walk a connection field.
func NewGraphQLRequest(ctx context.Context, endpoint string, gql GraphQLRequest) (*http.Request, error) {
	body, err := json.Marshal(gql)
	if err != nil {
		return nil, fmt.Errorf("httpclient: marshal graphql request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// GraphQLErrors mirrors the standard top-level `errors` array a GraphQL
// response includes on partial or total failure.
type GraphQLErrors []struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

func (e GraphQLErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Message
}

// ParseGraphQLErrors extracts the `errors` array from a decoded GraphQL
// response body, if present.
func ParseGraphQLErrors(decoded any) GraphQLErrors {
	raw, ok := getPath(decoded, "errors")
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var errs GraphQLErrors
	if json.Unmarshal(b, &errs) != nil {
		return nil
	}
	return errs
}

// GraphQLRelayPaginator implements the Relay Cursor Connections
// specification (`edges { node }`, `pageInfo { hasNextPage, endCursor }`),
// the de-facto standard cursor pagination shape for GraphQL APIs (GitHub,
// Shopify, Linear, ...): it reads pageInfo out of the response at
// DataPath + ".pageInfo" and, while hasNextPage is true, re-issues the
// same query with the `after` variable set to endCursor.
type GraphQLRelayPaginator struct {
	// DataPath locates the connection field in the response, e.g.
	// "data.repository.issues" or "data.search". pageInfo/edges are read
	// relative to this path.
	DataPath string
	// CursorVariable is the GraphQL variable name for the cursor, defaults to "after".
	CursorVariable string
	// PageSizeVariable, if set, is populated with PageSize on every request
	// (many schemas name their page-size argument "first").
	PageSizeVariable string
	PageSize         int

	bodyBytes []byte
}

func (p *GraphQLRelayPaginator) cursorVar() string {
	if p.CursorVariable == "" {
		return "after"
	}
	return p.CursorVariable
}

func (p *GraphQLRelayPaginator) Init(template *http.Request) (*http.Request, error) {
	body, err := bufferBody(template)
	if err != nil {
		return nil, err
	}
	p.bodyBytes = body

	if p.PageSizeVariable == "" || p.PageSize <= 0 {
		return template, nil
	}
	return cloneWithJSONBody(template, body, func(reqBody map[string]any) {
		setVariable(reqBody, p.PageSizeVariable, p.PageSize)
	})
}

func (p *GraphQLRelayPaginator) Next(_ context.Context, prev Page, _ int) (*http.Request, bool, error) {
	if errs := ParseGraphQLErrors(prev.Data); len(errs) > 0 {
		return nil, false, errs
	}

	pageInfoPath := p.DataPath
	if pageInfoPath != "" {
		pageInfoPath += ".pageInfo"
	} else {
		pageInfoPath = "pageInfo"
	}
	pageInfo, ok := getPath(prev.Data, pageInfoPath)
	if !ok {
		return nil, false, nil
	}
	hasNext, _ := getPath(pageInfo, "hasNextPage")
	if !asBool(hasNext) {
		return nil, false, nil
	}
	endCursorRaw, ok := getPath(pageInfo, "endCursor")
	if !ok {
		return nil, false, nil
	}
	endCursor, ok := asString(endCursorRaw)
	if !ok {
		return nil, false, nil
	}

	req, err := cloneWithJSONBody(prev.Response.Request, p.bodyBytes, func(body map[string]any) {
		setVariable(body, p.cursorVar(), endCursor)
		if p.PageSizeVariable != "" && p.PageSize > 0 {
			setVariable(body, p.PageSizeVariable, p.PageSize)
		}
	})
	return req, err == nil, err
}

func setVariable(body map[string]any, name string, value any) {
	vars, ok := body["variables"].(map[string]any)
	if !ok {
		vars = map[string]any{}
	}
	vars[name] = value
	body["variables"] = vars
}

// drainAndClose is a small helper connectors can use when they need to
// discard a response body they don't intend to read further.
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
