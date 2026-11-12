package server

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"horizon/applications/projects"
	"horizon/foundation/integrations/mcp"
	"horizon/foundation/runtime/app"
	"horizon/foundation/runtime/config"
	"horizon/foundation/runtime/transport"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/notifications"
	"horizon/foundation/system/settings"
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
	plugins := app.BuiltinPlugins(projects.New(core.Pool))
	if err := iam.ConfigurePluginPermissions(plugins.Catalog.Permissions); err != nil {
		return err
	}

	if err := plugins.Start(ctx); err != nil {
		return err
	}
	defer func() {
		cancelWorkers()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := plugins.Stop(stopCtx); err != nil {
			slog.Error("plugin shutdown failed", "error", err)
		}
	}()
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
	handler, start := transport.HandlerWithPlugins(app.System{}, core, plugins, iam)
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
