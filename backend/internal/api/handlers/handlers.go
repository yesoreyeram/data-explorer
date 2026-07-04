package handlers

import (
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/catalog"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/observability"
	"github.com/yesoreyeram/data-explorer/backend/internal/quota"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
)

type Handlers struct {
	Auth           *auth.Service
	AuthRepository *auth.Repository
	Audit          *audit.Service
	Connections    *connections.Service
	Workflows      *workflow.Service
	Catalog        *catalog.Service
	Metrics        *observability.Metrics
	Quotas         *quota.Service
	SecureCookies  bool
	RefreshTTL     time.Duration
}

func New(authSvc *auth.Service, authRepo *auth.Repository, auditSvc *audit.Service, connSvc *connections.Service, wfSvc *workflow.Service, catalogSvc *catalog.Service, metrics *observability.Metrics, quotas *quota.Service, secureCookies bool, refreshTTL time.Duration) *Handlers {
	return &Handlers{Auth: authSvc, AuthRepository: authRepo, Audit: auditSvc, Connections: connSvc, Workflows: wfSvc, Catalog: catalogSvc, Metrics: metrics, Quotas: quotas, SecureCookies: secureCookies, RefreshTTL: refreshTTL}
}

func guardrailTripKindsFromFrameWarnings(warnings []string) []string {
	seen := map[string]struct{}{}
	for _, warning := range warnings {
		lower := strings.ToLower(warning)
		switch {
		case strings.Contains(lower, "column cap"):
			seen["max_columns"] = struct{}{}
		case strings.Contains(lower, "cell") && strings.Contains(lower, "truncated"):
			seen["cell_bytes"] = struct{}{}
		case strings.Contains(lower, "row cap") || strings.Contains(lower, "row count is"):
			seen["max_rows"] = struct{}{}
		case strings.Contains(lower, "response body"):
			seen["http_response_body_bytes"] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	return out
}

func guardrailTripKindsFromError(err error) []string {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	seen := map[string]struct{}{}
	switch {
	case strings.Contains(msg, "maximum depth"):
		seen["json_depth"] = struct{}{}
	case strings.Contains(msg, "token limit"):
		seen["json_elements"] = struct{}{}
	case strings.Contains(msg, "decompression ratio"):
		seen["decompress_ratio"] = struct{}{}
	case strings.Contains(msg, "memory pressure"):
		seen["memory_pressure"] = struct{}{}
	case strings.Contains(msg, "quota") || strings.Contains(msg, "rate limit"):
		seen["quota"] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	return out
}
