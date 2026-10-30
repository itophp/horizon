package main

import (
	"baseline/services/api/db/migrations"
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"log"
	"os"
)

func main() {
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		log.Fatal(err)
	}
	if _, err = p.Up(context.Background()); err != nil {
		log.Fatal(err)
	}
}
