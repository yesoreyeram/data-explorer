// Command server is the entrypoint for the data-explorer API: it loads
// configuration, applies database migrations, wires every service and
// handler together, and serves HTTP until it receives a shutdown signal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/db"
	"github.com/yesoreyeram/data-explorer/backend/internal/adapters/ratelimit"
	"github.com/yesoreyeram/data-explorer/backend/internal/api"
	"github.com/yesoreyeram/data-explorer/backend/internal/api/handlers"
	custommw "github.com/yesoreyeram/data-explorer/backend/internal/api/middleware"
	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/catalog"
	"github.com/yesoreyeram/data-explorer/backend/internal/config"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections/connectors"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/observability"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/crypto"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/dbx"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/logger"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/migrator"
	"github.com/yesoreyeram/data-explorer/backend/internal/scheduler"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/egress"

	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	log.Info("starting data-explorer", "env", cfg.Env, "addr", cfg.HTTP.Addr)

	if err := httpx.ConfigureClientIP(cfg.HTTP.TrustedProxyMode); err != nil {
		return fmt.Errorf("configure client IP resolution: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.Connect(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := migrator.Apply(ctx, pool, db.MigrationsFS, "migrations", log); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	encryptor, err := crypto.NewEncryptor(cfg.Auth.EncryptionKeyBase64)
	if err != nil {
		return fmt.Errorf("init encryptor: %w", err)
	}

	// ---- Wire services ----
	authRepo := auth.NewRepository(pool)
	tokenManager := auth.NewTokenManager(cfg.Auth.JWTSigningKey, cfg.Auth.AccessTokenTTL)
	authSvc := auth.NewService(authRepo, tokenManager, cfg.Auth.RefreshTokenTTL)

	// Optional SSO: resolve each OIDC provider's discovery document at startup.
	if cfg.OIDC.ProvidersJSON != "" {
		var providers []auth.OIDCProviderConfig
		if err := json.Unmarshal([]byte(cfg.OIDC.ProvidersJSON), &providers); err != nil {
			return fmt.Errorf("parse OIDC_PROVIDERS: %w", err)
		}
		oidcMgr, err := auth.NewOIDCManager(ctx, providers)
		if err != nil {
			return fmt.Errorf("configure SSO: %w", err)
		}
		authSvc.SetOIDC(oidcMgr)
		log.Info("single sign-on enabled", "providers", len(providers))
	}

	auditSvc := audit.NewService(pool, log)

	// Build the SSRF egress guard and register every connector to dial through
	// it. The default policy blocks cloud metadata and loopback while still
	// permitting internal databases (see internal/config EgressConfig).
	baseGuard, err := egress.New(egress.Config{
		Mode:      egress.Mode(cfg.Egress.Policy),
		Allowlist: cfg.Egress.Allowlist,
	})
	if err != nil {
		return fmt.Errorf("configure egress guard: %w", err)
	}

	connectorTypes := []string{
		string(domain.ConnectionTypePostgres),
		string(domain.ConnectionTypeMySQL),
		string(domain.ConnectionTypeREST),
		string(domain.ConnectionTypeGraphQL),
		string(domain.ConnectionTypeAWS),
		string(domain.ConnectionTypeGCP),
		string(domain.ConnectionTypeAzure),
	}
	connectorRegistry := connections.NewRegistry()
	if err := connectors.RegisterAll(connectorRegistry, connectorTypes, connectors.Options{
		DialContext:   baseGuard.DialContext,
		StrictHeaders: true,
	}); err != nil {
		return fmt.Errorf("register connectors: %w", err)
	}

	connRepo := connections.NewRepository(pool)
	connSvc := connections.NewService(connRepo, encryptor, connectorRegistry)

	// The ad-hoc path dials arbitrary user-supplied targets; apply the
	// stricter egress policy there when one is configured.
	if cfg.Egress.PolicyAdhoc != "" {
		adhocGuard, err := egress.New(egress.Config{
			Mode:      egress.Mode(cfg.Egress.PolicyAdhoc),
			Allowlist: cfg.Egress.Allowlist,
		})
		if err != nil {
			return fmt.Errorf("configure ad-hoc egress guard: %w", err)
		}
		connSvc.SetAdhocDialContext(adhocGuard.DialContext)
	}

	nodeRegistry := nodes.DefaultRegistry()
	wfRepo := workflow.NewRepository(pool)
	wfEngine := workflow.NewEngine(nodeRegistry)
	wfSvc := workflow.NewService(wfRepo, wfEngine, connSvc)

	catalogSvc := catalog.NewService()

	metrics := observability.NewMetrics()

	// Rate limiters: shared (Redis) when REDIS_URL is set so a scaled
	// deployment enforces one budget across instances, else in-process.
	var generalLimiter, authLimiter custommw.Limiter
	if cfg.Redis.URL != "" {
		opt, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("parse REDIS_URL: %w", err)
		}
		rdb := redis.NewClient(opt)
		defer rdb.Close()
		generalLimiter = ratelimit.New(rdb, 20, 60, "rl:gen:", log)
		authLimiter = ratelimit.New(rdb, 2, 10, "rl:auth:", log)
		log.Info("using shared Redis rate limiter")
	} else {
		generalLimiter = custommw.NewIPRateLimiter(20, 60)
		authLimiter = custommw.NewIPRateLimiter(2, 10) // ~2 req/s, burst 10 - blunts credential stuffing
	}

	h := handlers.New(authSvc, authRepo, auditSvc, connSvc, wfSvc, catalogSvc, cfg.Env == "production", cfg.Auth.RefreshTokenTTL)
	h.OIDCPostLoginRedirect = cfg.OIDC.PostLoginRedirect
	healthHandler := handlers.NewHealthHandler(pool)

	router := api.NewRouter(cfg, h, healthHandler, tokenManager, metrics, generalLimiter, authLimiter)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.HTTP.RequestTimeout,
		WriteTimeout:      cfg.HTTP.RequestTimeout + 5*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sched := scheduler.New(wfSvc, log)
	go sched.Run(ctx)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("http server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Info("shutdown complete")
	return nil
}
