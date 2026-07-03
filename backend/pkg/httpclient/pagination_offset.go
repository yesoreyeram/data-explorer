package httpclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// OffsetLimitPaginator implements classic `?offset=0&limit=50`-style
// pagination: the offset advances by PageSize each page, and pagination
// stops once a page yields fewer than PageSize items (or zero).
type OffsetLimitPaginator struct {
	OffsetParam string // defaults to "offset"
	LimitParam  string // defaults to "limit"
	PageSize    int    // defaults to 50
	// ItemsPath locates the array of items within the JSON response body,
	// e.g. "data.items"; empty means the response body itself is the array.
	ItemsPath string

	bodyBytes []byte
	offset    int
}

func (p *OffsetLimitPaginator) params() (offsetParam, limitParam string, pageSize int) {
	offsetParam, limitParam, pageSize = p.OffsetParam, p.LimitParam, p.PageSize
	if offsetParam == "" {
		offsetParam = "offset"
	}
	if limitParam == "" {
		limitParam = "limit"
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	return
}

func (p *OffsetLimitPaginator) Init(template *http.Request) (*http.Request, error) {
	body, err := bufferBody(template)
	if err != nil {
		return nil, err
	}
	p.bodyBytes = body

	offsetParam, limitParam, pageSize := p.params()
	return cloneWithQuery(template, body, func(q url.Values) {
		q.Set(offsetParam, strconv.Itoa(0))
		q.Set(limitParam, strconv.Itoa(pageSize))
	}), nil
}

func (p *OffsetLimitPaginator) Next(_ context.Context, prev Page, _ int) (*http.Request, bool, error) {
	offsetParam, limitParam, pageSize := p.params()

	items, _ := getPath(prev.Data, p.ItemsPath)
	count := asArrayLen(items)
	if count == 0 || count < pageSize {
		return nil, false, nil
	}

	p.offset += pageSize
	req := cloneWithQuery(prev.Response.Request, p.bodyBytes, func(q url.Values) {
		q.Set(offsetParam, strconv.Itoa(p.offset))
		q.Set(limitParam, strconv.Itoa(pageSize))
	})
	return req, true, nil
}

// PagePaginator implements `?page=1&per_page=50`-style pagination: the page
// number increments each request, stopping once a page yields zero items.
type PagePaginator struct {
	PageParam     string // defaults to "page"
	PageSizeParam string // defaults to "per_page"
	PageSize      int    // defaults to 50
	StartPage     int    // defaults to 1
	ItemsPath     string

	bodyBytes []byte
	page      int
}

func (p *PagePaginator) params() (pageParam, sizeParam string, pageSize, startPage int) {
	pageParam, sizeParam, pageSize, startPage = p.PageParam, p.PageSizeParam, p.PageSize, p.StartPage
	if pageParam == "" {
		pageParam = "page"
	}
	if sizeParam == "" {
		sizeParam = "per_page"
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if startPage <= 0 {
		startPage = 1
	}
	return
}

func (p *PagePaginator) Init(template *http.Request) (*http.Request, error) {
	body, err := bufferBody(template)
	if err != nil {
		return nil, err
	}
	p.bodyBytes = body

	pageParam, sizeParam, pageSize, startPage := p.params()
	p.page = startPage
	return cloneWithQuery(template, body, func(q url.Values) {
		q.Set(pageParam, strconv.Itoa(startPage))
		q.Set(sizeParam, strconv.Itoa(pageSize))
	}), nil
}

func (p *PagePaginator) Next(_ context.Context, prev Page, _ int) (*http.Request, bool, error) {
	pageParam, sizeParam, pageSize, _ := p.params()

	items, _ := getPath(prev.Data, p.ItemsPath)
	if asArrayLen(items) == 0 {
		return nil, false, nil
	}

	p.page++
	req := cloneWithQuery(prev.Response.Request, p.bodyBytes, func(q url.Values) {
		q.Set(pageParam, strconv.Itoa(p.page))
		q.Set(sizeParam, strconv.Itoa(pageSize))
	})
	return req, true, nil
}
