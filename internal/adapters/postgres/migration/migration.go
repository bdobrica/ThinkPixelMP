// Package migration implements explicit, forward-only PostgreSQL schema upgrades.
// It is used by cmd/migrate, never by service startup.
package migration

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type step struct{ name, sql, checksum string }

// State is the validated state of one shipped migration.
type State struct {
	Name    string
	Applied bool
}

var filename = regexp.MustCompile(`^[0-9]{6}_[a-z][a-z0-9_]*\.sql$`)

func load(files fs.FS) ([]step, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	steps := make([]step, 0, len(names))
	for i, name := range names {
		if !filename.MatchString(name) || name[:6] != fmt.Sprintf("%06d", i+1) {
			return nil, fmt.Errorf("invalid migration sequence")
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("empty migration")
		}
		steps = append(steps, step{name, string(data), fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no migrations shipped")
	}
	return steps, nil
}

// Run checks history and optionally applies all pending migrations atomically.
// Callers provide a dedicated migration connection and a bounded context.
// Error details from PostgreSQL are deliberately not exposed (DSNs and SQL error
// details can contain credentials or tenant data).
func Run(ctx context.Context, conn *pgx.Conn, files fs.FS, apply bool) ([]State, error) {
	steps, err := load(files)
	if err != nil {
		return nil, fmt.Errorf("invalid shipped migrations")
	}
	opts := pgx.TxOptions{IsoLevel: pgx.ReadCommitted}
	if !apply {
		opts = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	}
	tx, err := conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin migration transaction failed")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if apply {
		// Transaction-scoped lock serializes concurrent migration commands, including
		// the first ledger creation. READ COMMITTED sees the previous runner's commit.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(6071225831888048205)`); err != nil {
			return nil, fmt.Errorf("acquire migration lock failed")
		}
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations (
   name text PRIMARY KEY, checksum text NOT NULL CHECK (length(checksum) = 64),
   applied_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
  )`); err != nil {
			return nil, fmt.Errorf("initialize migration history failed")
		}
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect migration history failed")
	}
	applied := map[string]string{}
	if exists {
		rows, err := tx.Query(ctx, `SELECT name, checksum FROM public.schema_migrations ORDER BY name`)
		if err != nil {
			return nil, fmt.Errorf("read migration history failed")
		}
		defer rows.Close()
		for rows.Next() {
			var name, checksum string
			if err := rows.Scan(&name, &checksum); err != nil {
				return nil, fmt.Errorf("decode migration history failed")
			}
			applied[name] = checksum
		}
		if rows.Err() != nil {
			return nil, fmt.Errorf("read migration history failed")
		}
	}
	states := make([]State, len(steps))
	pending := false
	for i, step := range steps {
		checksum, ok := applied[step.name]
		if ok && (pending || checksum != step.checksum) {
			return nil, fmt.Errorf("migration history drift detected")
		}
		if !ok {
			pending = true
		}
		states[i] = State{step.name, ok}
		delete(applied, step.name)
	}
	if len(applied) != 0 {
		return nil, fmt.Errorf("database contains unknown migrations")
	}
	if apply {
		for i, step := range steps {
			if states[i].Applied {
				continue
			}
			if _, err := tx.Exec(ctx, step.sql); err != nil {
				return nil, fmt.Errorf("apply migration %s failed", step.name)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.schema_migrations (name, checksum) VALUES ($1, $2)`, step.name, step.checksum); err != nil {
				return nil, fmt.Errorf("record migration failed")
			}
			states[i].Applied = true
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit migration transaction failed")
	}
	return states, nil
}
