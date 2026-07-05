package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/config"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

func TestRESTExecuteSimpleArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "name": "a"}, {"id": 2, "name": "b"}})
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(RESTConfig{BaseURL: srv.URL, AuthConfig: AuthConfig{AuthType: "bearer"}})
	rest := NewREST(Options{Guardrails: config.DefaultGuardrailsConfig()})

	frame, err := rest.Execute(context.Background(), cfgJSON, map[string]string{"bearerToken": "tok123"}, connections.QuerySpec{Method: "GET", RowLimit: 10})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if frame.NumRows() != 2 {
		t.Fatalf("expected 2 rows, got %d", frame.NumRows())
	}
	if frame.Meta.SourceType != "rest" {
		t.Fatalf("expected sourceType=rest, got %q", frame.Meta.SourceType)
	}
}

func TestRESTExecuteRejectsBadAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(RESTConfig{BaseURL: srv.URL})
	rest := NewREST(Options{Guardrails: config.DefaultGuardrailsConfig()})
	if _, err := rest.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{Method: "GET"}); err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestRESTExecuteWithOffsetPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		switch offset {
		case "0", "":
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": 1}, {"id": 2}}})
		case "2":
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": 3}}})
		default:
			t.Fatalf("unexpected offset %q", offset)
		}
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(RESTConfig{BaseURL: srv.URL})
	rest := NewREST(Options{Guardrails: config.DefaultGuardrailsConfig()})

	frame, err := rest.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{
		Method:   "GET",
		RowLimit: 100,
		Pagination: &connections.PaginationSpec{
			Strategy:  "offset",
			ItemsPath: "items",
			PageSize:  2,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if frame.NumRows() != 3 {
		t.Fatalf("expected 3 rows across 2 pages, got %d", frame.NumRows())
	}
}

func TestRESTExecuteEnforcesRowLimitAcrossPages(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"n": page}, {"n": page}}})
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(RESTConfig{BaseURL: srv.URL})
	rest := NewREST(Options{Guardrails: config.DefaultGuardrailsConfig()})

	frame, err := rest.Execute(context.Background(), cfgJSON, nil, connections.QuerySpec{
		Method:   "GET",
		RowLimit: 3,
		Pagination: &connections.PaginationSpec{
			Strategy:  "offset",
			ItemsPath: "items",
			PageSize:  2,
			MaxPages:  10,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if frame.NumRows() != 3 {
		t.Fatalf("expected row limit of 3 to be enforced across pages, got %d", frame.NumRows())
	}
	if !frame.Meta.Truncated {
		t.Fatal("expected Meta.Truncated to be set")
	}
}

func TestRESTConfigRejectsMissingBaseURL(t *testing.T) {
	rest := NewREST(Options{Guardrails: config.DefaultGuardrailsConfig()})
	_, err := rest.Execute(context.Background(), json.RawMessage(`{}`), nil, connections.QuerySpec{})
	if err == nil {
		t.Fatal("expected error for missing baseUrl")
	}
}
