package connections

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/crypto"
)

type Service struct {
	repo      *Repository
	encryptor *crypto.Encryptor
	registry  *Registry
}

func NewService(repo *Repository, encryptor *crypto.Encryptor, registry *Registry) *Service {
	return &Service{repo: repo, encryptor: encryptor, registry: registry}
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
func (s *Service) Test(ctx context.Context, id string) error {
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

	testErr := connector.Test(ctx, conn.Config, secret)
	errMsg := ""
	if testErr != nil {
		errMsg = testErr.Error()
	}
	if err := s.repo.SetTestResult(ctx, id, testErr == nil, errMsg); err != nil {
		return fmt.Errorf("record test result: %w", err)
	}
	return testErr
}

// Query executes a read query through the connection's connector. This is
// the single choke point used by both the ad-hoc "run query" API and the
// workflow source node executor.
func (s *Service) Query(ctx context.Context, id string, spec QuerySpec) (QueryResult, error) {
	conn, err := s.repo.Get(ctx, id)
	if err != nil {
		return QueryResult{}, err
	}
	connector, err := s.registry.Get(string(conn.Type))
	if err != nil {
		return QueryResult{}, err
	}
	secret, err := s.decryptSecretFor(ctx, id)
	if err != nil {
		return QueryResult{}, err
	}
	return connector.Execute(ctx, conn.Config, secret, spec)
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
