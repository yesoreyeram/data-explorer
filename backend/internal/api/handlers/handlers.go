// Package handlers implements the HTTP surface of the API: thin adapters
// that decode requests, call into the appropriate service, record an audit
// entry for anything that mutates state, and encode the response. Business
// logic itself lives in the service packages (auth, connections, workflow,
// audit), not here.
package handlers

import (
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
)

type Handlers struct {
	Auth           *auth.Service
	AuthRepository *auth.Repository
	Audit          *audit.Service
	Connections    *connections.Service
	Workflows      *workflow.Service
	SecureCookies  bool
	RefreshTTL     time.Duration
}

func New(authSvc *auth.Service, authRepo *auth.Repository, auditSvc *audit.Service, connSvc *connections.Service, wfSvc *workflow.Service, secureCookies bool, refreshTTL time.Duration) *Handlers {
	return &Handlers{
		Auth:           authSvc,
		AuthRepository: authRepo,
		Audit:          auditSvc,
		Connections:    connSvc,
		Workflows:      wfSvc,
		SecureCookies:  secureCookies,
		RefreshTTL:     refreshTTL,
	}
}
