package connections

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

var ErrNotFound = errors.New("connection not found")
var ErrConflict = errors.New("a connection with this name already exists")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const connectionColumns = `id, name, type, description, config, status, last_tested_at, last_error,
	last_error_code, last_error_remediation, last_check_duration_ms, created_by, created_at, updated_at`

func scanConnection(row interface {
	Scan(dest ...any) error
}) (domain.Connection, error) {
	var c domain.Connection
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.Config, &c.Status, &c.LastTestedAt, &c.LastError,
		&c.LastErrorCode, &c.LastErrorRemediation, &c.LastCheckDurationMs, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

type createParams struct {
	Name            string
	Type            domain.ConnectionType
	Description     string
	Config          json.RawMessage
	SecretEncrypted string
	CreatedBy       string
}

func (r *Repository) Create(ctx context.Context, p createParams) (domain.Connection, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO connections (name, type, description, config, secret_encrypted, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+connectionColumns,
		p.Name, p.Type, p.Description, p.Config, p.SecretEncrypted, p.CreatedBy,
	)
	c, err := scanConnection(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Connection{}, ErrConflict
		}
		return domain.Connection{}, fmt.Errorf("insert connection: %w", err)
	}
	return c, nil
}

func (r *Repository) List(ctx context.Context) ([]domain.Connection, error) {
	rows, err := r.db.Query(ctx, `SELECT `+connectionColumns+` FROM connections ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()

	var out []domain.Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Connection, error) {
	row := r.db.QueryRow(ctx, `SELECT `+connectionColumns+` FROM connections WHERE id = $1`, id)
	c, err := scanConnection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Connection{}, ErrNotFound
		}
		return domain.Connection{}, fmt.Errorf("query connection: %w", err)
	}
	return c, nil
}

// getSecret returns the raw encrypted secret blob for internal use only
// (decrypted exclusively inside Service, right before dialing out).
func (r *Repository) getSecret(ctx context.Context, id string) (string, error) {
	var secret string
	err := r.db.QueryRow(ctx, `SELECT secret_encrypted FROM connections WHERE id = $1`, id).Scan(&secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return secret, nil
}

type updateParams struct {
	Name        string
	Description string
	Config      json.RawMessage
	// SecretEncrypted is a pointer so callers can distinguish "leave the
	// existing secret untouched" (nil) from "replace it" (non-nil).
	SecretEncrypted *string
}

func (r *Repository) Update(ctx context.Context, id string, p updateParams) (domain.Connection, error) {
	var row interface {
		Scan(dest ...any) error
	}
	if p.SecretEncrypted != nil {
		row = r.db.QueryRow(ctx,
			`UPDATE connections SET name = $1, description = $2, config = $3, secret_encrypted = $4,
			 status = 'unverified', updated_at = now() WHERE id = $5
			 RETURNING `+connectionColumns,
			p.Name, p.Description, p.Config, *p.SecretEncrypted, id,
		)
	} else {
		row = r.db.QueryRow(ctx,
			`UPDATE connections SET name = $1, description = $2, config = $3,
			 status = 'unverified', updated_at = now() WHERE id = $4
			 RETURNING `+connectionColumns,
			p.Name, p.Description, p.Config, id,
		)
	}
	c, err := scanConnection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Connection{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Connection{}, ErrConflict
		}
		return domain.Connection{}, fmt.Errorf("update connection: %w", err)
	}
	return c, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM connections WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TestResult is the outcome of dialing out to a connection's underlying
// system - see connections.HealthError/Classify for where Code/Remediation
// come from.
type TestResult struct {
	Healthy          bool
	Error            string
	ErrorCode        string
	ErrorRemediation string
	DurationMs       int64
}

func (r *Repository) SetTestResult(ctx context.Context, id string, res TestResult) error {
	status := domain.ConnectionStatusHealthy
	if !res.Healthy {
		status = domain.ConnectionStatusUnhealthy
	}
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE connections SET status = $1, last_tested_at = $2, last_error = $3,
		 last_error_code = $4, last_error_remediation = $5, last_check_duration_ms = $6 WHERE id = $7`,
		status, now, res.Error, res.ErrorCode, res.ErrorRemediation, res.DurationMs, id,
	)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
