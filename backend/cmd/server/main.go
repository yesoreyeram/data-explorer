// Command server is the entrypoint for the data-explorer API: it loads
// configuration, applies database migrations, wires every service and
// handler together, and serves HTTP until it receives a shutdown signal.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/db"
	"github.com/yesoreyeram/data-explorer/backend/internal/api"
	"github.com/yesoreyeram/data-explorer/backend/internal/api/handlers"
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
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/logger"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/migrator"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
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

	auditSvc := audit.NewService(pool, log)

	connectorRegistry := connections.NewRegistry()
	connectorRegistry.Register(string(domain.ConnectionTypePostgres), connectors.NewPostgres())
	connectorRegistry.Register(string(domain.ConnectionTypeMySQL), connectors.NewMySQL())
	connectorRegistry.Register(string(domain.ConnectionTypeREST), connectors.NewREST())
	connectorRegistry.Register(string(domain.ConnectionTypeGraphQL), connectors.NewGraphQL())
	connectorRegistry.Register(string(domain.ConnectionTypeAWS), connectors.NewAWS())
	connectorRegistry.Register(string(domain.ConnectionTypeGCP), connectors.NewGCP())
	connectorRegistry.Register(string(domain.ConnectionTypeAzure), connectors.NewAzure())

	connRepo := connections.NewRepository(pool)
	connSvc := connections.NewService(connRepo, encryptor, connectorRegistry)

	nodeRegistry := nodes.DefaultRegistry()
	wfRepo := workflow.NewRepository(pool)
	wfEngine := workflow.NewEngine(nodeRegistry)
	wfSvc := workflow.NewService(wfRepo, wfEngine, connSvc)

	catalogSvc := catalog.NewService()

	metrics := observability.NewMetrics()

	h := handlers.New(authSvc, authRepo, auditSvc, connSvc, wfSvc, catalogSvc, cfg.Env == "production", cfg.Auth.RefreshTokenTTL)
	healthHandler := handlers.NewHealthHandler(pool)

	router := api.NewRouter(cfg, h, healthHandler, tokenManager, metrics)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.HTTP.RequestTimeout,
		WriteTimeout:      cfg.HTTP.RequestTimeout + 5*time.Second,
		IdleTimeout:       120 * time.Second,
	}

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
