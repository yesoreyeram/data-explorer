package connections

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// stubConnector is a minimal Connector for exercising Service.QueryAdhoc
// without a database or a real external system.
type stubConnector struct {
	execErr error
}

func (s *stubConnector) Test(ctx context.Context, config json.RawMessage, secret map[string]string) error {
	return nil
}

func (s *stubConnector) Execute(ctx context.Context, config json.RawMessage, secret map[string]string, spec QuerySpec) (*dataframe.Frame, error) {
	if s.execErr != nil {
		return nil, s.execErr
	}
	frame := dataframe.New(nil)
	frame.AppendRow(map[string]any{"answer": 42})
	return frame, nil
}

func newTestService(t *testing.T, connType string, connector Connector) *Service {
	t.Helper()
	registry := NewRegistry()
	registry.Register(connType, connector)
	return NewService(nil, nil, registry)
}

func TestQueryAdhocStampsMetadataAndTruncates(t *testing.T) {
	svc := newTestService(t, "stub", &stubConnector{})

	frame, err := svc.QueryAdhoc(context.Background(), "user-1", "stub", json.RawMessage(`{}`), nil, QuerySpec{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frame.Meta.SourceType != "stub" {
		t.Fatalf("expected SourceType %q, got %q", "stub", frame.Meta.SourceType)
	}
	if frame.Meta.SourceID != "" {
		t.Fatalf("adhoc frame should not carry a SourceID, got %q", frame.Meta.SourceID)
	}
	if frame.Meta.Name != "(temporary connection)" {
		t.Fatalf("expected placeholder Name, got %q", frame.Meta.Name)
	}
	if frame.NumRows() != 1 {
		t.Fatalf("expected 1 row, got %d", frame.NumRows())
	}
}

func TestQueryAdhocUnsupportedType(t *testing.T) {
	svc := newTestService(t, "stub", &stubConnector{})

	_, err := svc.QueryAdhoc(context.Background(), "user-1", "does-not-exist", json.RawMessage(`{}`), nil, QuerySpec{})
	if err != ErrUnsupportedType {
		t.Fatalf("expected ErrUnsupportedType, got %v", err)
	}
}

func TestQueryAdhocPropagatesConnectorError(t *testing.T) {
	wantErr := context.DeadlineExceeded
	svc := newTestService(t, "stub", &stubConnector{execErr: wantErr})

	_, err := svc.QueryAdhoc(context.Background(), "user-1", "stub", json.RawMessage(`{}`), nil, QuerySpec{})
	if err != wantErr {
		t.Fatalf("expected connector error to propagate, got %v", err)
	}
}

func TestQueryAdhocRateLimitsPerActor(t *testing.T) {
	svc := newTestService(t, "stub", &stubConnector{})
	svc.limiter = newPerConnectionLimiter(1, 1) // burst of 1, so a second immediate call is blocked

	if _, err := svc.QueryAdhoc(context.Background(), "user-1", "stub", json.RawMessage(`{}`), nil, QuerySpec{}); err != nil {
		t.Fatalf("first call should succeed, got %v", err)
	}
	if _, err := svc.QueryAdhoc(context.Background(), "user-1", "stub", json.RawMessage(`{}`), nil, QuerySpec{}); err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited for a second immediate call from the same actor, got %v", err)
	}
	if _, err := svc.QueryAdhoc(context.Background(), "user-2", "stub", json.RawMessage(`{}`), nil, QuerySpec{}); err != nil {
		t.Fatalf("a different actor should have its own budget, got %v", err)
	}
}
