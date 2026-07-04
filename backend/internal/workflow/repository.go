package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

const workflowColumns = `id, name, description, definition, status, version, created_by, created_at, updated_at,
	schedule_cron, schedule_enabled, schedule_next_run, schedule_last_run`

func scanWorkflow(row interface {
	Scan(dest ...any) error
}) (domain.Workflow, error) {
	var w domain.Workflow
	err := row.Scan(&w.ID, &w.Name, &w.Description, &w.Definition, &w.Status, &w.Version, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt,
		&w.ScheduleCron, &w.ScheduleEnabled, &w.ScheduleNextRun, &w.ScheduleLastRun)
	return w, err
}

var ErrNotFound = errors.New("workflow not found")
var ErrConflict = errors.New("a workflow with this name already exists")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, name, description string, definition json.RawMessage, createdBy string) (domain.Workflow, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO workflows (name, description, definition, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+workflowColumns,
		name, description, definition, createdBy,
	)
	w, err := scanWorkflow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Workflow{}, ErrConflict
		}
		return domain.Workflow{}, fmt.Errorf("insert workflow: %w", err)
	}
	return w, nil
}

func (r *Repository) List(ctx context.Context) ([]domain.Workflow, error) {
	rows, err := r.db.Query(ctx, `SELECT `+workflowColumns+` FROM workflows ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query workflows: %w", err)
	}
	defer rows.Close()

	var out []domain.Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Workflow, error) {
	row := r.db.QueryRow(ctx, `SELECT `+workflowColumns+` FROM workflows WHERE id = $1`, id)
	w, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Workflow{}, ErrNotFound
		}
		return domain.Workflow{}, fmt.Errorf("query workflow: %w", err)
	}
	return w, nil
}

func (r *Repository) Update(ctx context.Context, id, name, description string, definition json.RawMessage, status domain.WorkflowStatus) (domain.Workflow, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE workflows SET name = $1, description = $2, definition = $3, status = $4, version = version + 1, updated_at = now()
		 WHERE id = $5
		 RETURNING `+workflowColumns,
		name, description, definition, status, id,
	)
	w, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Workflow{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Workflow{}, ErrConflict
		}
		return domain.Workflow{}, fmt.Errorf("update workflow: %w", err)
	}
	return w, nil
}

// UpdateSchedule sets (or clears) a workflow's cron schedule. nextRun should
// already be computed by the caller (nil when enabled is false) - the
// repository layer doesn't parse cron expressions.
func (r *Repository) UpdateSchedule(ctx context.Context, id, cronExpr string, enabled bool, nextRun *time.Time) (domain.Workflow, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE workflows SET schedule_cron = $1, schedule_enabled = $2, schedule_next_run = $3
		 WHERE id = $4
		 RETURNING `+workflowColumns,
		cronExpr, enabled, nextRun, id,
	)
	w, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Workflow{}, ErrNotFound
		}
		return domain.Workflow{}, fmt.Errorf("update workflow schedule: %w", err)
	}
	return w, nil
}

// DueSchedules returns every enabled, scheduled workflow whose next run is
// at or before now - what the scheduler polls on each tick.
func (r *Repository) DueSchedules(ctx context.Context, now time.Time) ([]domain.Workflow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+workflowColumns+` FROM workflows WHERE schedule_enabled AND schedule_next_run <= $1`, now)
	if err != nil {
		return nil, fmt.Errorf("query due schedules: %w", err)
	}
	defer rows.Close()

	var out []domain.Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// MarkScheduleRun records that a scheduled run just happened and advances
// schedule_next_run so the same due row isn't picked up again next tick.
func (r *Repository) MarkScheduleRun(ctx context.Context, id string, lastRun time.Time, nextRun *time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflows SET schedule_last_run = $1, schedule_next_run = $2 WHERE id = $3`,
		lastRun, nextRun, id,
	)
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Executions ----

func (r *Repository) CreateExecution(ctx context.Context, workflowID, triggeredBy string) (domain.WorkflowExecution, error) {
	var ex domain.WorkflowExecution
	err := r.db.QueryRow(ctx,
		`INSERT INTO workflow_executions (workflow_id, triggered_by) VALUES ($1, $2)
		 RETURNING id, workflow_id, status, triggered_by, started_at, finished_at, duration_ms, error, node_results`,
		workflowID, triggeredBy,
	).Scan(&ex.ID, &ex.WorkflowID, &ex.Status, &ex.TriggeredBy, &ex.StartedAt, &ex.FinishedAt, &ex.DurationMs, &ex.Error, &ex.NodeResults)
	if err != nil {
		return domain.WorkflowExecution{}, fmt.Errorf("insert execution: %w", err)
	}
	return ex, nil
}

func (r *Repository) FinishExecution(ctx context.Context, id string, status domain.ExecutionStatus, durationMs int64, errMsg string, nodeResults json.RawMessage) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_executions SET status = $1, finished_at = now(), duration_ms = $2, error = $3, node_results = $4 WHERE id = $5`,
		status, durationMs, errMsg, nodeResults, id,
	)
	return err
}

func (r *Repository) ListExecutions(ctx context.Context, workflowID string, limit int) ([]domain.WorkflowExecution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, workflow_id, status, triggered_by, started_at, finished_at, duration_ms, error, node_results
		 FROM workflow_executions WHERE workflow_id = $1 ORDER BY started_at DESC LIMIT $2`,
		workflowID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query executions: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowExecution
	for rows.Next() {
		var ex domain.WorkflowExecution
		if err := rows.Scan(&ex.ID, &ex.WorkflowID, &ex.Status, &ex.TriggeredBy, &ex.StartedAt, &ex.FinishedAt, &ex.DurationMs, &ex.Error, &ex.NodeResults); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}

func (r *Repository) GetExecution(ctx context.Context, id string) (domain.WorkflowExecution, error) {
	var ex domain.WorkflowExecution
	err := r.db.QueryRow(ctx,
		`SELECT id, workflow_id, status, triggered_by, started_at, finished_at, duration_ms, error, node_results
		 FROM workflow_executions WHERE id = $1`, id,
	).Scan(&ex.ID, &ex.WorkflowID, &ex.Status, &ex.TriggeredBy, &ex.StartedAt, &ex.FinishedAt, &ex.DurationMs, &ex.Error, &ex.NodeResults)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WorkflowExecution{}, ErrNotFound
		}
		return domain.WorkflowExecution{}, fmt.Errorf("query execution: %w", err)
	}
	return ex, nil
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
