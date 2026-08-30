# Database migrations

Executable, immutable PostgreSQL migrations begin with the persistence foundation. The `cmd/migrate` command will be the explicit migration entry point; ordinary service startup will not silently mutate the schema.
