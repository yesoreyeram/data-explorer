package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

func TestGraphQLExecuteRelayEdges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"search": map[string]any{
					"edges": []map[string]any{
						{"node": map[string]any{"id": 1, "title": "one"}},
						{"node": map[string]any{"id": 2, "title": "two"}},
					},
				},
			},
		})
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(GraphQLConfig{Endpoint: srv.URL})
	g := NewGraphQL(Options{})

	frame, err := g.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{
		RowLimit: 100,
		GraphQL:  &connections.GraphQLSpec{Query: "query { search { edges { node { id title } } } }", DataPath: "data.search"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if frame.NumRows() != 2 {
		t.Fatalf("expected 2 rows (unwrapped from edges/node), got %d", frame.NumRows())
	}
	if frame.Rows()[0]["title"] != "one" {
		t.Fatalf("expected node fields unwrapped directly onto the row, got %+v", frame.Rows()[0])
	}
}

func TestGraphQLExecutePropagatesGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "field not found"}},
		})
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(GraphQLConfig{Endpoint: srv.URL})
	g := NewGraphQL(Options{})

	_, err := g.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{
		GraphQL: &connections.GraphQLSpec{Query: "query { bogus }"},
	})
	if err == nil {
		t.Fatal("expected graphql errors array to surface as an error")
	}
}

func TestGraphQLExecuteWithRelayPagination(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		after, _ := vars["after"].(string)

		if after == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"search": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "c1"},
						"edges":    []map[string]any{{"node": map[string]any{"id": 1}}},
					},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"search": map[string]any{
					"pageInfo": map[string]any{"hasNextPage": false},
					"edges":    []map[string]any{{"node": map[string]any{"id": 2}}},
				},
			},
		})
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(GraphQLConfig{Endpoint: srv.URL})
	g := NewGraphQL(Options{})

	frame, err := g.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{
		RowLimit: 100,
		GraphQL:  &connections.GraphQLSpec{Query: "query($after:String){ search(after:$after) { edges { node { id } } pageInfo { hasNextPage endCursor } } }", DataPath: "data.search"},
		Pagination: &connections.PaginationSpec{
			Strategy: "graphqlRelay",
			MaxPages: 10,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if frame.NumRows() != 2 {
		t.Fatalf("expected 2 rows across both pages, got %d", frame.NumRows())
	}
}
