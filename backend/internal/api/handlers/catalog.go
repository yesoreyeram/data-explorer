package handlers

import (
	"net/http"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

func (h *Handlers) ListCatalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entries := h.Catalog.Search(q.Get("q"), q.Get("category"), q.Get("type"))
	httpx.WriteJSON(w, http.StatusOK, entries)
}
