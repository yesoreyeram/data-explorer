// Package config loads application configuration from environment variables,
// following twelve-factor app principles. There is no config file parsing on
// purpose: environment variables are the single source of truth, which keeps
// behavior identical across local dev, CI, and containers.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env    string // development | production | test
	HTTP   HTTPConfig
	DB     DBConfig
	Auth   AuthConfig
	Log    LogConfig
	Egress EgressConfig
}

// EgressConfig controls the SSRF egress guard applied to every outbound
// connector dial. See pkg/egress for the policy semantics.
type EgressConfig struct {
	// Policy: allow-private (default) | allowlist | public-only. The default
	// permits internal databases but always blocks cloud metadata and
	// loopback - the targets no connector legitimately needs.
	Policy string
	// Allowlist holds host[:port] patterns for the allowlist policy.
	Allowlist []string
	// PolicyAdhoc optionally applies a stricter policy to the ad-hoc query
	// path (temporary connections dial fully arbitrary targets). Empty means
	// "same as Policy".
	PolicyAdhoc string
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
			DSN:             getEnv("DATABASE_URL", "postgres://data_explorer:data_explorer@localhost:5432/data_explorer?sslmode=disable"),
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
		Egress: EgressConfig{
			Policy:      getEnv("EGRESS_POLICY", "allow-private"),
			Allowlist:   splitCSV(getEnv("EGRESS_ALLOWLIST", "")),
			PolicyAdhoc: getEnv("EGRESS_POLICY_ADHOC", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
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
		// Safe, clearly-marked default for local development only.
		c.Auth.JWTSigningKey = "dev-only-insecure-signing-key-change-me-32bytes"
	}
	if c.Auth.EncryptionKeyBase64 == "" {
		// Fixed (not random) so that connection secrets encrypted in one dev
		// session can still be decrypted after a restart. Base64 of 32 zero-ish
		// dev-marker bytes - never use this outside local development.
		c.Auth.EncryptionKeyBase64 = "ZGV2LW9ubHktaW5zZWN1cmUtMzJieXRlLWtleSEhISE="
	}
	if err := validateEgressPolicy(c.Egress.Policy, c.Egress.Allowlist); err != nil {
		return err
	}
	if c.Egress.PolicyAdhoc != "" {
		if err := validateEgressPolicy(c.Egress.PolicyAdhoc, c.Egress.Allowlist); err != nil {
			return err
		}
	}
	return nil
}

func validateEgressPolicy(policy string, allowlist []string) error {
	switch policy {
	case "allow-private", "public-only":
	case "allowlist":
		if len(allowlist) == 0 {
			return fmt.Errorf("EGRESS_ALLOWLIST must be set when the egress policy is 'allowlist'")
		}
	default:
		return fmt.Errorf("egress policy %q is not valid (allow-private | allowlist | public-only)", policy)
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
