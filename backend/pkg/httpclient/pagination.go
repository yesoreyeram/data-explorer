package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DefaultMaxPages is the guardrail applied when a caller doesn't specify
// one: enough for the overwhelming majority of legitimate paginated
// listings, low enough that a misconfigured "next page" detection (e.g. a
// cursor field that never goes empty) can't loop indefinitely.
const DefaultMaxPages = 20

// HardMaxPages is an absolute ceiling no caller can raise past, protecting
// the server even from a deliberately-misconfigured high MaxPages.
const HardMaxPages = 500

// Page is one fetched page: the raw response plus (if the body was valid
// JSON) its decoded form, which paginators inspect to find the next
// cursor/link/offset.
type Page struct {
	Response *Response
	Data     any // decoded JSON body, or nil if the body wasn't JSON
}

// Paginator decides, given the page just fetched, what request (if any)
// retrieves the next page. Implementations are stateless with respect to
// page count - the driving loop (Client.DoPaginated) owns guardrails like
// MaxPages so every strategy gets them for free.
type Paginator interface {
	// Init returns the first request to send, derived from a template
	// request the caller has already built (method, URL, headers, body).
	Init(template *http.Request) (*http.Request, error)
	// Next returns the request for the page after prev, or ok=false if
	// prev was the last page.
	Next(ctx context.Context, prev Page, pageIndex int) (req *http.Request, ok bool, err error)
}

// PaginationResult aggregates every page fetched by DoPaginated.
type PaginationResult struct {
	Pages     []Page
	PageCount int
	// Truncated is true if MaxPages was hit before the paginator reported
	// it was done.
	Truncated bool
}

// DoPaginated drives a Paginator to completion (or until maxPages is hit),
// issuing one request per page through c. A maxPages <= 0 uses
// DefaultMaxPages; any value is clamped to HardMaxPages.
func (c *Client) DoPaginated(ctx context.Context, template *http.Request, paginator Paginator, maxPages int) (*PaginationResult, error) {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if maxPages > HardMaxPages {
		maxPages = HardMaxPages
	}

	req, err := paginator.Init(template)
	if err != nil {
		return nil, fmt.Errorf("httpclient: init pagination: %w", err)
	}

	result := &PaginationResult{}

	for i := 0; i < maxPages; i++ {
		resp, err := c.Do(ctx, req)
		if err != nil {
			return result, err
		}

		page := Page{Response: resp}
		if len(resp.Body) > 0 {
			var decoded any
			if json.Unmarshal(resp.Body, &decoded) == nil {
				page.Data = decoded
			}
		}
		result.Pages = append(result.Pages, page)
		result.PageCount++

		if resp.IsError() {
			// Stop paginating on an error response, but return what we have
			// so the caller can decide how to surface a partial result.
			return result, nil
		}

		next, ok, err := paginator.Next(ctx, page, i)
		if err != nil {
			return result, fmt.Errorf("httpclient: determine next page: %w", err)
		}
		if !ok {
			return result, nil
		}
		req = next
	}

	result.Truncated = true
	return result, nil
}
