package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CursorPaginator implements opaque-cursor pagination, the most common
// style for modern REST APIs (Stripe, Slack, ...): each response embeds a
// token identifying where the next page starts; the client echoes it back
// as a query parameter on the next request until the field is absent/empty.
type CursorPaginator struct {
	// CursorParam is the query parameter name to send the cursor as on
	// subsequent requests, e.g. "cursor" or "starting_after".
	CursorParam string
	// CursorPath locates the next-cursor value in the JSON response body,
	// e.g. "meta.next_cursor" or "paging.cursors.after".
	CursorPath string

	bodyBytes []byte
}

func (p *CursorPaginator) Init(template *http.Request) (*http.Request, error) {
	body, err := bufferBody(template)
	if err != nil {
		return nil, err
	}
	p.bodyBytes = body
	return template, nil
}

func (p *CursorPaginator) Next(_ context.Context, prev Page, _ int) (*http.Request, bool, error) {
	raw, ok := getPath(prev.Data, p.CursorPath)
	if !ok {
		return nil, false, nil
	}
	cursor, ok := asString(raw)
	if !ok || cursor == "" {
		return nil, false, nil
	}

	param := p.CursorParam
	if param == "" {
		param = "cursor"
	}
	req := cloneWithQuery(prev.Response.Request, p.bodyBytes, func(q url.Values) {
		q.Set(param, cursor)
	})
	return req, true, nil
}

// LinkHeaderPaginator implements RFC 5988 Link-header pagination (GitHub's
// REST API is the canonical example): the server returns the next page's
// full URL in a `Link: <url>; rel="next"` response header, which the client
// follows verbatim until no `rel="next"` link is present.
type LinkHeaderPaginator struct {
	bodyBytes []byte
}

func (p *LinkHeaderPaginator) Init(template *http.Request) (*http.Request, error) {
	body, err := bufferBody(template)
	if err != nil {
		return nil, err
	}
	p.bodyBytes = body
	return template, nil
}

func (p *LinkHeaderPaginator) Next(_ context.Context, prev Page, _ int) (*http.Request, bool, error) {
	nextURL, ok := parseLinkHeaderNext(prev.Response.Header.Get("Link"))
	if !ok {
		return nil, false, nil
	}

	parsed, err := url.Parse(nextURL)
	if err != nil {
		return nil, false, err
	}
	req := prev.Response.Request.Clone(prev.Response.Request.Context())
	req.URL = parsed
	if p.bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(p.bodyBytes))
	}
	return req, true, nil
}

// parseLinkHeaderNext extracts the rel="next" URL from an RFC 5988 Link
// header value, e.g. `<https://api.example.com/x?page=2>; rel="next", <...>; rel="last"`.
// Splitting on "," is a simplification (RFC 5988 link-values are themselves
// comma-separated and never contain a literal comma in practice for the
// APIs this targets), which keeps the parser dependency-free.
func parseLinkHeaderNext(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		start := strings.IndexByte(part, '<')
		end := strings.IndexByte(part, '>')
		if start < 0 || end < 0 || end <= start {
			continue
		}
		linkURL := part[start+1 : end]
		rest := part[end+1:]
		if strings.Contains(rest, `rel="next"`) || strings.Contains(rest, "rel=next") {
			return linkURL, true
		}
	}
	return "", false
}
