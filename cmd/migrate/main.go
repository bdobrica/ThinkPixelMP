package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/migration"
	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/migrations"
	"github.com/jackc/pgx/v5"
)

const usage = `Usage: migrate status|up

status validates and reports migration history without changing the database.
up applies pending migrations atomically. Released SQL is immutable; repairs use
new forward migrations. Service startup never changes the schema.

Set TPMP_MIGRATION_DATABASE_URL_REF to env:NAME or file:/absolute/path containing
the administrative PostgreSQL connection URL. The service credential is not used.`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	case "status", "up":
	default:
		fmt.Fprintf(stderr, "unknown migration command %q\n\n%s\n", args[0], usage)
		return 2
	}
	ref, err := config.ParseSecretRef(os.Getenv("TPMP_MIGRATION_DATABASE_URL_REF"))
	if err != nil {
		fmt.Fprintln(stderr, "migration database secret reference is required or invalid")
		return 1
	}
	secret, err := ref.Resolve(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(stderr, "migration database secret is unavailable")
		return 1
	}
	cfg, err := pgx.ParseConfig(secret.Value())
	if err != nil {
		fmt.Fprintln(stderr, "migration database configuration is invalid")
		return 1
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["application_name"] = "thinkpixelmp-migrate"
	cfg.RuntimeParams["search_path"] = "pg_catalog, public"
	cfg.RuntimeParams["statement_timeout"] = "60000"
	cfg.RuntimeParams["lock_timeout"] = "10000"
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signals, 2*time.Minute)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		fmt.Fprintln(stderr, "migration database connection failed")
		return 1
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(cleanup)
	}()
	states, err := migration.Run(ctx, conn, migrations.Files, args[0] == "up")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, state := range states {
		status := "pending"
		if state.Applied {
			status = "applied"
		}
		fmt.Fprintf(stdout, "%s %s\n", state.Name, status)
	}
	return 0
}
