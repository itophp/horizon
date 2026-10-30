package server

import (
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/config"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/repository"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/internal/settings"
	"baseline/services/api/internal/transport"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"time"
)

func Run(ctx context.Context, c config.Config) error {
	var store app.RecordStore
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
		store = repository.Records{Queries: dbgen.New(pool)}
		identityPool = pool
	}
	engine, err := authorization.New()
	if err != nil {
		return err
	}
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	iam := identity.New(identityPool, c.SecureCookies)
	core := governance.New(identityPool, iam, c.Governance)
	iam.Policies = core
	if identityPool != nil {
		fileService := files.New(core)
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
					if e := fileService.Purge(ctx); e != nil {
						slog.Error("file purge failed", "code", governance.Code(e))
					}
				}
			}
		}()
	}
	server := &http.Server{Addr: c.Address, Handler: transport.HandlerWithCore(app.System{}, app.Demo{Store: store, Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{Token: c.DemoToken}, engine, core, iam), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
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
