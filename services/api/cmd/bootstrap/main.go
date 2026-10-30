package main

import (
	"baseline/services/api/internal/identity"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	username, password := os.Getenv("BOOTSTRAP_USERNAME"), os.Getenv("BOOTSTRAP_PASSWORD")
	tenant := os.Getenv("BOOTSTRAP_TENANT")
	if tenant == "" {
		tenant = "workspace-default"
	}
	if url == "" || username == "" || password == "" {
		log.Fatal("DATABASE_URL, BOOTSTRAP_USERNAME, BOOTSTRAP_PASSWORD are required")
	}
	ctx := context.Background()
	if err := identity.Migrate(ctx, url); err != nil {
		log.Fatal("migration failed")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer pool.Close()
	account, err := identity.Bootstrap(ctx, pool, tenant, username, password)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Administrator created: %s (password was not logged)\n", account.Username)
}
