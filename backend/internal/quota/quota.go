package quota

import (
	"sync"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/config"
)

type Kind string

const (
	KindExplore  Kind = "explore"
	KindWorkflow Kind = "workflow"
)

type Result struct {
	Allowed    bool
	Quota      int
	Used       int
	Window     time.Duration
	RetryAfter time.Duration
}

type Service struct {
	cfg    config.GuardrailsConfig
	window time.Duration
	state  sync.Map
}

type bucket struct {
	mu   sync.Mutex
	hits []time.Time
}

func NewService(cfg config.GuardrailsConfig) *Service {
	return &Service{cfg: cfg, window: time.Hour}
}

func (s *Service) Check(userID string, roles []string, kind Kind, now time.Time) Result {
	quota := s.quotaFor(roles, kind)
	if quota <= 0 || userID == "" {
		return Result{Allowed: true, Window: s.window}
	}
	key := string(kind) + ":" + userID
	value, _ := s.state.LoadOrStore(key, &bucket{})
	b := value.(*bucket)
	cutoff := now.Add(-s.window)
	b.mu.Lock()
	defer b.mu.Unlock()
	kept := b.hits[:0]
	for _, ts := range b.hits {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	b.hits = kept
	used := len(b.hits)
	if used >= quota {
		retryAfter := s.window
		if used > 0 {
			retryAfter = b.hits[0].Add(s.window).Sub(now)
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
		}
		return Result{Allowed: false, Quota: quota, Used: used, Window: s.window, RetryAfter: retryAfter}
	}
	b.hits = append(b.hits, now)
	return Result{Allowed: true, Quota: quota, Used: len(b.hits), Window: s.window}
}

func (s *Service) quotaFor(roles []string, kind Kind) int {
	best := 0
	for _, role := range roles {
		q, ok := s.cfg.RoleQuotas[role]
		if !ok {
			continue
		}
		candidate := 0
		switch kind {
		case KindExplore:
			candidate = q.ExploreRunsPerHour
		case KindWorkflow:
			candidate = q.WorkflowRunsPerHour
		}
		if candidate > best {
			best = candidate
		}
	}
	return best
}
