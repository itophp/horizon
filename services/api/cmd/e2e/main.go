package main

import (
	"baseline/services/api/internal/config"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/server"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	source := os.Getenv("TEST_DATABASE_URL")
	if source == "" {
		return fmt.Errorf("TEST_DATABASE_URL must point to isolated local/CI PostgreSQL")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	admin, err := pgx.Connect(ctx, source)
	if err != nil {
		return err
	}
	defer admin.Close(context.Background())
	schema := fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		return err
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			slog.Error("test schema cleanup failed", "error", err)
		}
	}()
	parsed, err := url.Parse(source)
	if err != nil {
		return err
	}
	query := parsed.Query()
	// Tables stay in the disposable schema; public keeps shared extension objects
	// (pg_trgm operators used by knowledge retrieval) resolvable.
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	dsn := parsed.String()
	if err = identity.Migrate(ctx, dsn); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	// These credentials exist only inside an isolated, disposable test schema.
	if _, err = identity.Bootstrap(ctx, pool, "e2e-workspace", "admin", "TestOnly!2026Secure"); err != nil {
		return err
	}
	address := os.Getenv("API_ADDR")
	if address == "" {
		address = "127.0.0.1:8099"
	}
	root, err := os.MkdirTemp("", "baseline-e2e-storage-")
	if err != nil {
		return err
	}
	defer func() {
		if e := os.RemoveAll(root); e != nil {
			slog.Error("test file cleanup failed", "error_type", fmt.Sprintf("%T", e))
		}
	}()
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return err
	}
	return server.Run(ctx, config.Config{Address: address, DatabaseURL: dsn, Governance: governance.Options{EncryptionKey: key, StorageRoot: root, PublicWorkspace: "e2e-workspace"}})
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
