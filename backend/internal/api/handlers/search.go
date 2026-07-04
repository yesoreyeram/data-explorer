package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

type searchResult struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Href string `json:"href"`
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	p, ok := rbac.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "a valid access token is required")
		return
	}
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	results := []searchResult{{Type: "page", Name: "Dashboard", Href: "/"}}
	if p.Has(rbac.PermConnectionsRead) {
		results = append(results, searchResult{Type: "page", Name: "Explore", Href: "/explore"}, searchResult{Type: "page", Name: "Connections", Href: "/connections"})
		if conns, err := h.Connections.List(r.Context()); err == nil {
			for _, conn := range conns {
				if q == "" || strings.Contains(strings.ToLower(conn.Name), q) {
					results = append(results, searchResult{Type: "connection", ID: conn.ID, Name: conn.Name, Href: "/connections"})
				}
			}
		}
	}
	if p.Has(rbac.PermWorkflowsRead) {
		results = append(results, searchResult{Type: "page", Name: "Workflows", Href: "/workflows"})
		if workflows, err := h.Workflows.List(r.Context()); err == nil {
			for _, wf := range workflows {
				if q == "" || strings.Contains(strings.ToLower(wf.Name), q) {
					results = append(results, searchResult{Type: "workflow", ID: wf.ID, Name: wf.Name, Href: "/workflows/" + wf.ID})
				}
			}
		}
	}
	if p.Has(rbac.PermAuditRead) {
		results = append(results, searchResult{Type: "page", Name: "Audit Log", Href: "/audit-log"})
	}
	if q != "" {
		filtered := results[:0]
		for _, result := range results {
			if strings.Contains(strings.ToLower(result.Name), q) || strings.Contains(strings.ToLower(result.Type), q) {
				filtered = append(filtered, result)
			}
		}
		results = filtered
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	if len(results) > 10 {
		results = results[:10]
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}
