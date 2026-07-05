// Package config loads application configuration from environment variables,
// with an optional JSON guardrails file for operator-tuned platform limits.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env        string // development | production | test
	HTTP       HTTPConfig
	DB         DBConfig
	Auth       AuthConfig
	Log        LogConfig
	Guardrails GuardrailsConfig
}

type HTTPConfig struct {
	Addr            string
	AllowedOrigins  []string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type AuthConfig struct {
	// JWTSigningKey signs access & refresh tokens. Must be >= 32 bytes in production.
	JWTSigningKey   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	// EncryptionKey is a 32-byte (AES-256) key used to encrypt connection secrets at rest.
	EncryptionKeyBase64 string
}

type LogConfig struct {
	Level  string
	Format string // json | text
}

type GuardrailsConfig struct {
	MaxBodyBytes       int64                `json:"max_body_bytes"`
	MaxRows            int                  `json:"max_rows"`
	MaxRedirects       int                  `json:"max_redirects"`
	MaxPages           int                  `json:"max_pages"`
	RunTimeout         time.Duration        `json:"run_timeout_seconds"`
	NodeTimeout        time.Duration        `json:"node_timeout_seconds"`
	MaxColumns         int                  `json:"max_columns"`
	MaxStringCellBytes int                  `json:"max_string_cell_bytes"`
	MaxBytesCellBytes  int                  `json:"max_bytes_cell_bytes"`
	JSONMaxDepth       int                  `json:"json_max_depth"`
	JSONMaxElements    int                  `json:"json_max_elements"`
	DecompressRatio    int                  `json:"decompress_ratio"`
	DictThreshold      int                  `json:"dict_threshold"`
	RoleQuotas         map[string]RoleQuota `json:"role_quotas"`
}

type RoleQuota struct {
	ExploreRunsPerHour  int `json:"explore_runs_per_hour"`
	WorkflowRunsPerHour int `json:"workflow_runs_per_hour"`
}

func DefaultGuardrailsConfig() GuardrailsConfig {
	return GuardrailsConfig{
		MaxBodyBytes:       25 * 1024 * 1024,
		MaxRows:            10_000,
		MaxRedirects:       5,
		MaxPages:           20,
		RunTimeout:         2 * time.Minute,
		NodeTimeout:        60 * time.Second,
		MaxColumns:         512,
		MaxStringCellBytes: 1 * 1024 * 1024,
		MaxBytesCellBytes:  5 * 1024 * 1024,
		JSONMaxDepth:       64,
		JSONMaxElements:    5_000_000,
		DecompressRatio:    100,
		DictThreshold:      128,
		RoleQuotas:         map[string]RoleQuota{},
	}
}

func Load() (*Config, error) {
	cfg := &Config{
		Env: getEnv("APP_ENV", "development"),
		HTTP: HTTPConfig{
			Addr:            getEnv("HTTP_ADDR", ":8080"),
			AllowedOrigins:  splitCSV(getEnv("HTTP_ALLOWED_ORIGINS", "http://localhost:5173")),
			ShutdownTimeout: getDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			RequestTimeout:  getDuration("HTTP_REQUEST_TIMEOUT", 30*time.Second),
		},
		DB: DBConfig{
			DSN:             getEnv("DATABASE_URL", "******localhost:5432/data_explorer?sslmode=disable"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Auth: AuthConfig{
			JWTSigningKey:       getEnv("JWT_SIGNING_KEY", ""),
			AccessTokenTTL:      getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL:     getDuration("REFRESH_TOKEN_TTL", 168*time.Hour),
			EncryptionKeyBase64: getEnv("CONNECTION_ENCRYPTION_KEY", ""),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Guardrails: DefaultGuardrailsConfig(),
	}
	if path := strings.TrimSpace(os.Getenv("GUARDRAILS_CONFIG_FILE")); path != "" {
		if err := cfg.loadGuardrailsFile(path); err != nil {
			return nil, err
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) loadGuardrailsFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read GUARDRAILS_CONFIG_FILE: %w", err)
	}
	var raw struct {
		MaxBodyBytes       *int64               `json:"max_body_bytes"`
		MaxRows            *int                 `json:"max_rows"`
		MaxRedirects       *int                 `json:"max_redirects"`
		MaxPages           *int                 `json:"max_pages"`
		RunTimeoutSeconds  *int                 `json:"run_timeout_seconds"`
		NodeTimeoutSeconds *int                 `json:"node_timeout_seconds"`
		MaxColumns         *int                 `json:"max_columns"`
		MaxStringCellBytes *int                 `json:"max_string_cell_bytes"`
		MaxBytesCellBytes  *int                 `json:"max_bytes_cell_bytes"`
		JSONMaxDepth       *int                 `json:"json_max_depth"`
		JSONMaxElements    *int                 `json:"json_max_elements"`
		DecompressRatio    *int                 `json:"decompress_ratio"`
		DictThreshold      *int                 `json:"dict_threshold"`
		RoleQuotas         map[string]RoleQuota `json:"role_quotas"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse GUARDRAILS_CONFIG_FILE: %w", err)
	}
	if raw.MaxBodyBytes != nil {
		c.Guardrails.MaxBodyBytes = *raw.MaxBodyBytes
	}
	if raw.MaxRows != nil {
		c.Guardrails.MaxRows = *raw.MaxRows
	}
	if raw.MaxRedirects != nil {
		c.Guardrails.MaxRedirects = *raw.MaxRedirects
	}
	if raw.MaxPages != nil {
		c.Guardrails.MaxPages = *raw.MaxPages
	}
	if raw.RunTimeoutSeconds != nil {
		c.Guardrails.RunTimeout = time.Duration(*raw.RunTimeoutSeconds) * time.Second
	}
	if raw.NodeTimeoutSeconds != nil {
		c.Guardrails.NodeTimeout = time.Duration(*raw.NodeTimeoutSeconds) * time.Second
	}
	if raw.MaxColumns != nil {
		c.Guardrails.MaxColumns = *raw.MaxColumns
	}
	if raw.MaxStringCellBytes != nil {
		c.Guardrails.MaxStringCellBytes = *raw.MaxStringCellBytes
	}
	if raw.MaxBytesCellBytes != nil {
		c.Guardrails.MaxBytesCellBytes = *raw.MaxBytesCellBytes
	}
	if raw.JSONMaxDepth != nil {
		c.Guardrails.JSONMaxDepth = *raw.JSONMaxDepth
	}
	if raw.JSONMaxElements != nil {
		c.Guardrails.JSONMaxElements = *raw.JSONMaxElements
	}
	if raw.DecompressRatio != nil {
		c.Guardrails.DecompressRatio = *raw.DecompressRatio
	}
	if raw.DictThreshold != nil {
		c.Guardrails.DictThreshold = *raw.DictThreshold
	}
	if raw.RoleQuotas != nil {
		c.Guardrails.RoleQuotas = raw.RoleQuotas
	}
	return nil
}

func (c *Config) validate() error {
	if c.Env == "production" {
		if len(c.Auth.JWTSigningKey) < 32 {
			return fmt.Errorf("JWT_SIGNING_KEY must be set and at least 32 bytes in production")
		}
		if c.Auth.EncryptionKeyBase64 == "" {
			return fmt.Errorf("CONNECTION_ENCRYPTION_KEY must be set in production")
		}
	}
	if c.Auth.JWTSigningKey == "" {
		c.Auth.JWTSigningKey = "dev-only-insecure-signing-key-change-me-32bytes"
	}
	if c.Auth.EncryptionKeyBase64 == "" {
		c.Auth.EncryptionKeyBase64 = "ZGV2LW9ubHktaW5zZWN1cmUtMzJieXRlLWtleSEhISE="
	}
	if c.Guardrails.MaxBodyBytes <= 0 || c.Guardrails.MaxRows <= 0 || c.Guardrails.MaxColumns <= 0 {
		return fmt.Errorf("guardrail limits must be positive")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
