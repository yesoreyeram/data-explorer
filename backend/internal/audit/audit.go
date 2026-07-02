// Package audit records who did what, to which resource, from where, and
// whether it succeeded. Every mutating API call and every security-sensitive
// read (e.g. viewing decrypted connection tests, exporting data) goes through
// Service.Record. Writes are best-effort and never block or fail the
// request they describe: an audit outage should degrade observability, not
// availability. It is intentionally append-only (no update/delete
// endpoints) to preserve evidentiary integrity.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

type Event struct {
	ActorID      string
	ActorEmail   string
	Action       string // e.g. "connection.create", "user.login", "workflow.execute"
	ResourceType string // e.g. "connection", "workflow", "user"
	ResourceID   string
	IPAddress    string
	UserAgent    string
	Outcome      Outcome
	Metadata     map[string]any
}

type Service struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewService(db *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{db: db, log: log}
}

func (s *Service) Record(ctx context.Context, evt Event) {
	metadataJSON, err := json.Marshal(evt.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}
	outcome := evt.Outcome
	if outcome == "" {
		outcome = OutcomeSuccess
	}

	// Detached context: an inbound request cancellation must not drop the
	// audit trail for an action that already happened.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	_, err = s.db.Exec(writeCtx,
		`INSERT INTO audit_logs (actor_id, actor_email, action, resource_type, resource_id, ip_address, user_agent, metadata, outcome)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		evt.ActorID, evt.ActorEmail, evt.Action, evt.ResourceType, evt.ResourceID, evt.IPAddress, evt.UserAgent, metadataJSON, outcome,
	)
	if err != nil {
		s.log.Error("failed to write audit log", "error", err, "action", evt.Action, "resource_type", evt.ResourceType)
	}
}

type ListFilter struct {
	ActorID      string
	Action       string
	ResourceType string
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Offset       int
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]domain.AuditLog, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	where := "WHERE 1=1"
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.ActorID != "" {
		where += " AND actor_id = " + arg(f.ActorID)
	}
	if f.Action != "" {
		where += " AND action = " + arg(f.Action)
	}
	if f.ResourceType != "" {
		where += " AND resource_type = " + arg(f.ResourceType)
	}
	if f.Since != nil {
		where += " AND created_at >= " + arg(*f.Since)
	}
	if f.Until != nil {
		where += " AND created_at <= " + arg(*f.Until)
	}

	var total int
	countQuery := "SELECT count(*) FROM audit_logs " + where
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	limitArg := arg(f.Limit)
	offsetArg := arg(f.Offset)
	query := fmt.Sprintf(
		`SELECT id, actor_id, actor_email, action, resource_type, resource_id, ip_address, user_agent, metadata, outcome, created_at
		 FROM audit_logs %s ORDER BY created_at DESC LIMIT %s OFFSET %s`, where, limitArg, offsetArg)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorEmail, &a.Action, &a.ResourceType, &a.ResourceID, &a.IPAddress, &a.UserAgent, &a.Metadata, &a.Outcome, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
