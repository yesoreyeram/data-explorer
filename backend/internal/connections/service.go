package connections

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/config"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/crypto"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

var ErrRateLimited = fmt.Errorf("this connection is being called too frequently; slow down")

type Service struct {
	repo       *Repository
	encryptor  *crypto.Encryptor
	registry   *Registry
	limiter    *perConnectionLimiter
	guardrails config.GuardrailsConfig
}

func NewService(repo *Repository, encryptor *crypto.Encryptor, registry *Registry, guardrails config.GuardrailsConfig) *Service {
	return &Service{repo: repo, encryptor: encryptor, registry: registry, limiter: newPerConnectionLimiter(DefaultConnectionRateLimit, DefaultConnectionRateBurst), guardrails: guardrails}
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
	return s.repo.Create(ctx, createParams{Name: in.Name, Type: in.Type, Description: in.Description, Config: in.Config, SecretEncrypted: encrypted, CreatedBy: in.CreatedBy})
}

func (s *Service) List(ctx context.Context) ([]domain.Connection, error) { return s.repo.List(ctx) }
func (s *Service) Get(ctx context.Context, id string) (domain.Connection, error) {
	return s.repo.Get(ctx, id)
}

type UpdateInput struct {
	Name        string
	Description string
	Config      json.RawMessage
	Secret      map[string]string
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

func (s *Service) Delete(ctx context.Context, id string) error { return s.repo.Delete(ctx, id) }

func (s *Service) Test(ctx context.Context, id string) (TestResult, error) {
	if !s.limiter.Allow(id) {
		return TestResult{}, ErrRateLimited
	}
	conn, err := s.repo.Get(ctx, id)
	if err != nil {
		return TestResult{}, err
	}
	connector, err := s.registry.Get(string(conn.Type))
	if err != nil {
		return TestResult{}, err
	}
	secret, err := s.decryptSecretFor(ctx, id)
	if err != nil {
		return TestResult{}, err
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
	testedAt, err := s.repo.SetTestResult(ctx, id, result)
	if err != nil {
		return TestResult{}, fmt.Errorf("record test result: %w", err)
	}
	result.LastTestedAt = testedAt
	result.Status = domain.ConnectionStatusHealthy
	if !result.Healthy {
		result.Status = domain.ConnectionStatusUnhealthy
	}
	if classified != nil {
		return result, classified
	}
	return result, nil
}

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
	s.applyFrameGuardrails(frame)
	return frame, nil
}

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
	s.applyFrameGuardrails(frame)
	return frame, nil
}

func (s *Service) applyFrameGuardrails(frame *dataframe.Frame) {
	if frame == nil {
		return
	}
	frame.LimitColumns(s.guardrails.MaxColumns)
	frame.TruncateCellsByType(s.guardrails.MaxStringCellBytes, s.guardrails.MaxBytesCellBytes)
	frame.ApplyDictionaryEncoding(s.guardrails.DictThreshold)
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
