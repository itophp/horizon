package server

import (
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/config"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/files"
	"baseline/services/api/internal/modules/mcp"
	"baseline/services/api/internal/modules/notifications"
	"baseline/services/api/internal/modules/settings"
	"baseline/services/api/internal/transport"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"time"
)

func Run(ctx context.Context, c config.Config) error {
	var identityPool *pgxpool.Pool
	if c.DatabaseURL != "" {
		dbctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(dbctx, c.DatabaseURL)
		if err != nil {
			return errors.New("database configuration invalid")
		}
		defer pool.Close()
		if err = pool.Ping(dbctx); err != nil {
			return errors.New("database connection failed")
		}
		identityPool = pool
	}
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	iam := identity.New(identityPool, c.SecureCookies)
	core := governance.New(identityPool, iam, c.Governance)
	iam.Policies = core
	if identityPool != nil {
		fileService := files.New(core)
		mcpService := mcp.New(identityPool, iam)
		worker := notifications.New(core)
		go worker.Run(workerCtx)
		go settings.New(core).RunScheduled(workerCtx)
		go func() {
			timer := time.NewTicker(time.Minute)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					if e := mcpService.Purge(workerCtx); e != nil {
						slog.Error("MCP retention purge failed", "code", "internal")
					}
					if e := fileService.Purge(ctx); e != nil {
						slog.Error("file purge failed", "code", governance.Code(e))
					}
				}
			}
		}()
	}
	handler, start := transport.HandlerWithRuntime(app.System{}, core, iam)
	if identityPool != nil {
		go start(workerCtx)
	}
	server := &http.Server{Addr: c.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	done := make(chan error, 1)
	go func() { slog.Info("api starting", "address", c.Address); done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
