package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

type Postgres struct{ opts Options }

func NewPostgres(opts Options) *Postgres { return &Postgres{opts: opts} }

var validPostgresSSLModes = map[string]struct{}{
	"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {},
}

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
	// DSN hygiene: reject values that could smuggle an alternate target past
	// the egress guard (unix socket, multi-host DSN, an invalid sslmode).
	if strings.TrimSpace(cfg.Host) == "" {
		return "", connections.NewConfigError("Host is required.")
	}
	if strings.ContainsAny(cfg.Host, "/,@ ") {
		return "", connections.NewConfigError("Host must be a single hostname or IP address.")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return "", connections.NewConfigError("Port must be between 1 and 65535.")
	}
	if _, ok := validPostgresSSLModes[cfg.SSLMode]; !ok {
		return "", connections.NewConfigError("SSL mode is not valid.")
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

// connect builds a pgx connection whose dialer is the egress guard, so the
// database host is validated and pinned at dial time.
func (p *Postgres) connect(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) (*pgx.Conn, error) {
	dsn, err := p.dsn(cfgJSON, secret)
	if err != nil {
		return nil, err
	}
	pcfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}
	if dial := p.opts.dial(ctx); dial != nil {
		pcfg.DialFunc = dial
	}
	return pgx.ConnectConfig(ctx, pcfg)
}

func (p *Postgres) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := p.connect(ctx, cfgJSON, secret)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

func (p *Postgres) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	start := time.Now()
	sqlText, err := projectedReadOnlySQL(spec.SQL, spec.ProjectionHint)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := p.connect(ctx, cfgJSON, secret)
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
