package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// MaxExecutionDuration bounds how long a single workflow run may take,
// protecting the server from a runaway pipeline (e.g. a huge join fan-out).
const MaxExecutionDuration = 2 * time.Minute

type Service struct {
	repo             *Repository
	engine           *Engine
	connections      *connections.Service
	inFlight         atomic.Int64
	activeMu         sync.Mutex
	activeByWorkflow map[string]int
}

func NewService(repo *Repository, engine *Engine, connSvc *connections.Service) *Service {
	return &Service{repo: repo, engine: engine, connections: connSvc, activeByWorkflow: make(map[string]int)}
}

func (s *Service) Create(ctx context.Context, name, description string, definition json.RawMessage, createdBy string) (domain.Workflow, error) {
	def, err := ParseDefinition(definition)
	if err != nil {
		return domain.Workflow{}, err
	}
	if err := def.Validate(); err != nil {
		return domain.Workflow{}, fmt.Errorf("invalid workflow definition: %w", err)
	}
	if len(definition) == 0 {
		definition = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	return s.repo.Create(ctx, name, description, definition, createdBy)
}

func (s *Service) List(ctx context.Context) ([]domain.Workflow, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Workflow, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id, name, description string, definition json.RawMessage, status domain.WorkflowStatus) (domain.Workflow, error) {
	def, err := ParseDefinition(definition)
	if err != nil {
		return domain.Workflow{}, err
	}
	if err := def.Validate(); err != nil {
		return domain.Workflow{}, fmt.Errorf("invalid workflow definition: %w", err)
	}
	return s.repo.Update(ctx, id, name, description, definition, status)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// SetSchedule validates cronExpr (when enabled) and persists it, computing
// the next run time from now so the scheduler's next poll picks it up.
// Disabling (enabled=false) keeps the stored cron string so re-enabling
// doesn't require retyping it, but clears schedule_next_run.
func (s *Service) SetSchedule(ctx context.Context, id, cronExpr string, enabled bool) (domain.Workflow, error) {
	var nextRun *time.Time
	if enabled {
		if _, err := s.repo.Get(ctx, id); err != nil {
			return domain.Workflow{}, err
		}
		next, err := NextRun(cronExpr, time.Now())
		if err != nil {
			return domain.Workflow{}, err
		}
		nextRun = &next
	}
	return s.repo.UpdateSchedule(ctx, id, cronExpr, enabled, nextRun)
}

// DueSchedules and MarkScheduleRun back internal/scheduler's poll loop -
// routed through the service (not the repository directly) so schedule
// reads/writes go through the same layer as everything else, even though
// today they're thin passthroughs.

func (s *Service) DueSchedules(ctx context.Context) ([]domain.Workflow, error) {
	return s.repo.DueSchedules(ctx, time.Now())
}

func (s *Service) MarkScheduleRun(ctx context.Context, id string, lastRun time.Time, nextRun *time.Time) error {
	return s.repo.MarkScheduleRun(ctx, id, lastRun, nextRun)
}

func (s *Service) ListExecutions(ctx context.Context, workflowID string, limit int) ([]domain.WorkflowExecution, error) {
	return s.repo.ListExecutions(ctx, workflowID, limit)
}

func (s *Service) GetExecution(ctx context.Context, id string) (domain.WorkflowExecution, error) {
	return s.repo.GetExecution(ctx, id)
}

func (s *Service) InFlightRuns() int {
	return int(s.inFlight.Load())
}

func (s *Service) IsWorkflowInFlight(workflowID string) bool {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return s.activeByWorkflow[workflowID] > 0
}

func (s *Service) RecordSkippedSchedule(ctx context.Context, workflowID, reason string) error {
	return s.repo.CreateSkippedExecution(ctx, workflowID, "scheduler", reason)
}

// Execute runs the workflow's current definition end-to-end and persists a
// WorkflowExecution record (with per-node timing/row counts) regardless of
// whether the run succeeds or fails, so the execution history always
// reflects reality.
func (s *Service) Execute(ctx context.Context, workflowID, triggeredBy string) (domain.WorkflowExecution, *dataframe.Frame, error) {
	s.inFlight.Add(1)
	defer s.inFlight.Add(-1)

	wf, err := s.repo.Get(ctx, workflowID)
	if err != nil {
		return domain.WorkflowExecution{}, nil, err
	}

	def, err := ParseDefinition(wf.Definition)
	if err != nil {
		return domain.WorkflowExecution{}, nil, err
	}
	if err := def.Validate(); err != nil {
		return domain.WorkflowExecution{}, nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	execRecord, err := s.repo.CreateExecution(ctx, workflowID, triggeredBy)
	if err != nil {
		return domain.WorkflowExecution{}, nil, err
	}
	s.activeMu.Lock()
	s.activeByWorkflow[workflowID]++
	s.activeMu.Unlock()
	defer func() {
		s.activeMu.Lock()
		s.activeByWorkflow[workflowID]--
		if s.activeByWorkflow[workflowID] <= 0 {
			delete(s.activeByWorkflow, workflowID)
		}
		s.activeMu.Unlock()
	}()

	runCtx, cancel := context.WithTimeout(ctx, MaxExecutionDuration)
	defer cancel()

	start := time.Now()
	runResult, runErr := s.engine.Run(runCtx, def, nodes.Deps{Connections: s.connections})
	duration := time.Since(start)

	status := domain.ExecutionStatusSucceeded
	errMsg := ""
	if runErr != nil {
		status = domain.ExecutionStatusFailed
		errMsg = runErr.Error()
	}

	nodeResultsJSON, _ := json.Marshal(runResult.NodeResults)
	if err := s.repo.FinishExecution(ctx, execRecord.ID, status, duration.Milliseconds(), errMsg, nodeResultsJSON); err != nil {
		return domain.WorkflowExecution{}, nil, fmt.Errorf("record execution result: %w", err)
	}

	execRecord.Status = status
	execRecord.Error = errMsg
	execRecord.DurationMs = duration.Milliseconds()
	execRecord.NodeResults = nodeResultsJSON

	return execRecord, runResult.Output, runErr
}
