package identity

import (
	"baseline/services/api/db/migrations"
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Migrate(ctx context.Context, url string) error {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
