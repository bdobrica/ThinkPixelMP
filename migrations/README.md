# Database migrations

Executable, immutable PostgreSQL migrations begin with DB-001. The `cmd/migrate`
command is the explicit migration entry point; ordinary service startup does not
silently mutate the schema. Until DB-001 adds the migration framework and initial
tenant schema, `make migrate` reports an empty migration set without connecting to
or changing PostgreSQL.
