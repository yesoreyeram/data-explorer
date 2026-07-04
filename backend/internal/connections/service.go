package connections

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/crypto"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

var ErrRateLimited = fmt.Errorf("this connection is being called too frequently; slow down")

type Service struct {
	repo      *Repository
	encryptor *crypto.Encryptor
	registry  *Registry
	limiter   *perConnectionLimiter
}

func NewService(repo *Repository, encryptor *crypto.Encryptor, registry *Registry) *Service {
	return &Service{
		repo:      repo,
		encryptor: encryptor,
		registry:  registry,
		limiter:   newPerConnectionLimiter(DefaultConnectionRateLimit, DefaultConnectionRateBurst),
	}
}

type CreateInput struct {
	Name        string
	Type        domain.ConnectionType
	Description string
	Config      json.RawMessage
	Secret      map[string]string
	CreatedBy   string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Connection, error) {
	if _, err := s.registry.Get(string(in.Type)); err != nil {
		return domain.Connection{}, err
	}

	encrypted, err := s.encryptSecret(in.Secret)
	if err != nil {
		return domain.Connection{}, err
	}

	return s.repo.Create(ctx, createParams{
		Name:            in.Name,
		Type:            in.Type,
		Description:     in.Description,
		Config:          in.Config,
		SecretEncrypted: encrypted,
		CreatedBy:       in.CreatedBy,
	})
}

func (s *Service) List(ctx context.Context) ([]domain.Connection, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Connection, error) {
	return s.repo.Get(ctx, id)
}

type UpdateInput struct {
	Name        string
	Description string
	Config      json.RawMessage
	// Secret is nil when the caller does not want to change the stored secret.
	Secret map[string]string
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (domain.Connection, error) {
	params := updateParams{Name: in.Name, Description: in.Description, Config: in.Config}
	if in.Secret != nil {
		encrypted, err := s.encryptSecret(in.Secret)
		if err != nil {
			return domain.Connection{}, err
		}
		params.SecretEncrypted = &encrypted
	}
	return s.repo.Update(ctx, id, params)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Test dials out to the connection's underlying system to verify
// connectivity/credentials and records the result on the connection record.
// A failure is classified (see HealthError/Classify) before it's persisted
// or returned, so callers get a stable code and an actionable next step
// instead of just whatever string the underlying driver/SDK produced.
func (s *Service) Test(ctx context.Context, id string) error {
	if !s.limiter.Allow(id) {
		return ErrRateLimited
	}

	conn, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	connector, err := s.registry.Get(string(conn.Type))
	if err != nil {
		return err
	}
	secret, err := s.decryptSecretFor(ctx, id)
	if err != nil {
		return err
	}

	start := time.Now()
	testErr := connector.Test(ctx, conn.Config, secret)
	duration := time.Since(start).Milliseconds()

	result := TestResult{Healthy: testErr == nil, DurationMs: duration}
	var classified *HealthError
	if testErr != nil {
		classified = Classify(testErr)
		result.Error = classified.Message
		result.ErrorCode = string(classified.Code)
		result.ErrorRemediation = classified.Remediation
	}
	if err := s.repo.SetTestResult(ctx, id, result); err != nil {
		return fmt.Errorf("record test result: %w", err)
	}
	if classified != nil {
		return classified
	}
	return nil
}

// Query executes a read query through the connection's connector and
// returns a dataframe.Frame - the single choke point used by both the
// ad-hoc "run query" API and the workflow source node executor. The
// connector itself doesn't know the connection's ID/display name, so
// Query stamps that provenance onto the frame's metadata here.
func (s *Service) Query(ctx context.Context, id string, spec QuerySpec) (*dataframe.Frame, error) {
	if !s.limiter.Allow(id) {
		return nil, ErrRateLimited
	}

	conn, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	connector, err := s.registry.Get(string(conn.Type))
	if err != nil {
		return nil, err
	}
	secret, err := s.decryptSecretFor(ctx, id)
	if err != nil {
		return nil, err
	}

	frame, err := connector.Execute(ctx, conn.Config, secret, spec)
	if err != nil {
		return nil, Classify(err)
	}

	frame.Meta.SourceType = string(conn.Type)
	frame.Meta.SourceID = conn.ID
	frame.Meta.Name = conn.Name

	// Guardrail applied uniformly regardless of which connector produced the
	// frame: a single oversized cell (a REST blob field, a Postgres text
	// column with an enormous value, ...) shouldn't dominate memory/response
	// size just because the row count itself was within limits.
	frame.TruncateCells(dataframe.DefaultMaxCellBytes)

	return frame, nil
}

// QueryAdhoc executes a read query against a connection definition that is
// never persisted - config and secret exist only in memory for the duration
// of this call. It powers the "temporary connection" mode of the ad-hoc
// exploration page; querying a saved connection instead goes through Query.
// actorID rate-limits per caller, since there's no connection ID to key the
// per-connection limiter on.
func (s *Service) QueryAdhoc(ctx context.Context, actorID, connType string, config json.RawMessage, secret map[string]string, spec QuerySpec) (*dataframe.Frame, error) {
	if !s.limiter.Allow("adhoc:" + actorID) {
		return nil, ErrRateLimited
	}

	connector, err := s.registry.Get(connType)
	if err != nil {
		return nil, err
	}

	frame, err := connector.Execute(ctx, config, secret, spec)
	if err != nil {
		return nil, Classify(err)
	}

	frame.Meta.SourceType = connType
	frame.Meta.Name = "(temporary connection)"
	frame.TruncateCells(dataframe.DefaultMaxCellBytes)

	return frame, nil
}

func (s *Service) encryptSecret(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	plaintext, err := json.Marshal(domain.ConnectionSecret{Values: values})
	if err != nil {
		return "", fmt.Errorf("marshal secret: %w", err)
	}
	encrypted, err := s.encryptor.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %w", err)
	}
	return encrypted, nil
}

func (s *Service) decryptSecretFor(ctx context.Context, id string) (map[string]string, error) {
	encrypted, err := s.repo.getSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	if encrypted == "" {
		return map[string]string{}, nil
	}
	plaintext, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	var secret domain.ConnectionSecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return nil, fmt.Errorf("unmarshal secret: %w", err)
	}
	return secret.Values, nil
}
