package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOffsetLimitPaginator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		var items []int
		switch offset {
		case "0":
			items = []int{1, 2}
		case "2":
			items = []int{3}
		default:
			t.Fatalf("unexpected offset %q", offset)
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	paginator := &OffsetLimitPaginator{PageSize: 2}

	result, err := c.DoPaginated(context.Background(), req, paginator, 10)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("expected 2 pages, got %d", result.PageCount)
	}
	if result.Truncated {
		t.Fatal("did not expect truncation")
	}
}

func TestPagePaginatorStopsOnEmptyPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		if page == "1" {
			json.NewEncoder(w).Encode([]int{1})
			return
		}
		json.NewEncoder(w).Encode([]int{})
	}))
	defer srv.Close()

	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	result, err := c.DoPaginated(context.Background(), req, &PagePaginator{PageSize: 10}, 10)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("expected 2 pages (1 with data, 1 empty stopping condition), got %d", result.PageCount)
	}
}

func TestCursorPaginator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		switch cursor {
		case "":
			json.NewEncoder(w).Encode(map[string]any{"items": []int{1, 2}, "next_cursor": "page2"})
		case "page2":
			json.NewEncoder(w).Encode(map[string]any{"items": []int{3}, "next_cursor": ""})
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer srv.Close()

	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	paginator := &CursorPaginator{CursorParam: "cursor", CursorPath: "next_cursor"}

	result, err := c.DoPaginated(context.Background(), req, paginator, 10)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("expected 2 pages, got %d", result.PageCount)
	}
}

func TestLinkHeaderPaginator(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			w.Write([]byte(`{"ok":true}`))
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, srv.URL))
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	result, err := c.DoPaginated(context.Background(), req, &LinkHeaderPaginator{}, 10)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("expected 2 pages, got %d", result.PageCount)
	}
}

func TestGraphQLRelayPaginator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		after, _ := vars["after"].(string)

		switch after {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"search": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "cursor1"},
						"edges":    []any{map[string]any{"node": map[string]any{"id": 1}}},
					},
				},
			})
		case "cursor1":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"search": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
						"edges":    []any{map[string]any{"node": map[string]any{"id": 2}}},
					},
				},
			})
		default:
			t.Fatalf("unexpected cursor %q", after)
		}
	}))
	defer srv.Close()

	c := New(Config{})
	req, err := NewGraphQLRequest(context.Background(), srv.URL, GraphQLRequest{Query: "query { search { edges { node { id } } pageInfo { hasNextPage endCursor } } }"})
	if err != nil {
		t.Fatalf("NewGraphQLRequest: %v", err)
	}

	paginator := &GraphQLRelayPaginator{DataPath: "data.search"}
	result, err := c.DoPaginated(context.Background(), req, paginator, 10)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("expected 2 pages, got %d", result.PageCount)
	}
}

func TestDoPaginatedTruncatesAtMaxPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"next_cursor": "always-more"})
	}))
	defer srv.Close()

	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	paginator := &CursorPaginator{CursorPath: "next_cursor"}

	result, err := c.DoPaginated(context.Background(), req, paginator, 3)
	if err != nil {
		t.Fatalf("DoPaginated: %v", err)
	}
	if result.PageCount != 3 || !result.Truncated {
		t.Fatalf("expected truncation at 3 pages, got count=%d truncated=%v", result.PageCount, result.Truncated)
	}
}
