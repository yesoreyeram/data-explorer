package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
}

type MySQL struct{ opts Options }

// The MySQL driver's dialer registry is process-global and keyed by a "net"
// name, so the guarded dialer is registered once. Its closure resolves the
// effective egress dialer from the request context (a per-call override for
// the ad-hoc path) or the base guard captured at registration.
const mysqlGuardedNet = "de-guarded-tcp"

var (
	mysqlRegisterOnce sync.Once
	mysqlBaseDial     httpclient.DialFunc
)

func NewMySQL(opts Options) *MySQL {
	if opts.DialContext != nil {
		mysqlRegisterOnce.Do(func() {
			mysqlBaseDial = opts.DialContext
			mysqldriver.RegisterDialContext(mysqlGuardedNet, func(ctx context.Context, addr string) (net.Conn, error) {
				dial := mysqlBaseDial
				if override := connections.DialContextFrom(ctx); override != nil {
					dial = override
				}
				return dial(ctx, "tcp", addr)
			})
		})
	}
	return &MySQL{opts: opts}
}

func (m *MySQL) dsn(cfgJSON json.RawMessage, secret map[string]string) (string, error) {
	var cfg MySQLConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return "", fmt.Errorf("invalid mysql config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return "", connections.NewConfigError("Host is required.")
	}
	if strings.ContainsAny(cfg.Host, "/,@ ") {
		return "", connections.NewConfigError("Host must be a single hostname or IP address.")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return "", connections.NewConfigError("Port must be between 1 and 65535.")
	}
	mysqlCfg := mysqldriver.NewConfig()
	mysqlCfg.User = cfg.User
	mysqlCfg.Passwd = secret["password"]
	mysqlCfg.Net = "tcp"
	if m.opts.DialContext != nil {
		mysqlCfg.Net = mysqlGuardedNet
	}
	mysqlCfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.Timeout = 10 * time.Second
	mysqlCfg.ParseTime = true
	return mysqlCfg.FormatDSN(), nil
}

func (m *MySQL) open(cfgJSON json.RawMessage, secret map[string]string) (*sql.DB, error) {
	dsn, err := m.dsn(cfgJSON, secret)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(30 * time.Second)
	return db, nil
}

func (m *MySQL) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	db, err := m.open(cfgJSON, secret)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func (m *MySQL) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	start := time.Now()
	if err := EnsureReadOnlySQL(spec.SQL); err != nil {
		return nil, err
	}

	db, err := m.open(cfgJSON, secret)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, spec.SQL, spec.Params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}

	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)

	scanDest := make([]any, len(columns))
	scanBuf := make([]any, len(columns))
	for i := range scanBuf {
		scanDest[i] = &scanBuf[i]
	}

	rowCount := 0
	truncated := false
	for rows.Next() {
		if rowCount >= limit {
			truncated = true
			break
		}
		if err := rows.Scan(scanDest...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeMySQLValue(scanBuf[i])
		}
		frame.AppendRow(row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	frame.SetMeta(dataframe.Metadata{
		SourceType:  "mysql",
		GeneratedAt: start,
		DurationMs:  time.Since(start).Milliseconds(),
		Truncated:   truncated,
	})
	return frame, nil
}

// normalizeMySQLValue converts driver-returned []byte (common for numeric/
// text types under this driver) into plain strings so results serialize to
// clean JSON instead of base64 byte arrays.
func normalizeMySQLValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}
