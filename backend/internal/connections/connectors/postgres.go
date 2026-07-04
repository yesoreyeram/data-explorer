package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	SSLMode  string `json:"sslMode"`
}

type Postgres struct{}

func NewPostgres() *Postgres { return &Postgres{} }

func (p *Postgres) dsn(cfgJSON json.RawMessage, secret map[string]string) (string, error) {
	var cfg PostgresConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return "", fmt.Errorf("invalid postgres config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "prefer"
	}
	password := secret["password"]
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     "/" + cfg.Database,
		RawQuery: "sslmode=" + url.QueryEscape(cfg.SSLMode),
	}
	return u.String(), nil
}

func (p *Postgres) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	dsn, err := p.dsn(cfgJSON, secret)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

func (p *Postgres) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	start := time.Now()
	if err := EnsureReadOnlySQL(spec.SQL); err != nil {
		return nil, err
	}
	sqlText := applyProjectionHint(spec.SQL, spec.ProjectionHint)

	dsn, err := p.dsn(cfgJSON, secret)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	// Belt-and-braces: cap server-side execution time too, in case a
	// read-only SELECT is still expensive (e.g. missing index).
	if _, err := conn.Exec(ctx, "SET statement_timeout = '25s'"); err != nil {
		return nil, fmt.Errorf("set statement_timeout: %w", err)
	}

	rows, err := conn.Query(ctx, sqlText, spec.Params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)

	var columns []string
	for _, f := range rows.FieldDescriptions() {
		columns = append(columns, string(f.Name))
	}

	rowCount := 0
	truncated := false
	for rows.Next() {
		if rowCount >= limit {
			truncated = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			if i < len(values) {
				row[col] = normalizePostgresValue(values[i])
			}
		}
		frame.AppendRow(row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	frame.SetMeta(dataframe.Metadata{
		SourceType:  "postgres",
		GeneratedAt: start,
		DurationMs:  time.Since(start).Milliseconds(),
		Truncated:   truncated,
	})
	return frame, nil
}

// normalizePostgresValue fixes up driver return types that are correct for
// Go but awkward as JSON: pgx decodes uuid columns to a raw [16]byte array,
// which would otherwise serialize as an unreadable array of integers
// instead of the canonical "xxxxxxxx-xxxx-..." string every client expects.
func normalizePostgresValue(v any) any {
	if b, ok := v.([16]byte); ok {
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	return v
}
