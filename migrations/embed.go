// Package migrations contains the immutable SQL shipped with the migration command.
package migrations

import "embed"

// Files contains the ordered forward-only migrations.
//
//go:embed *.sql
var Files embed.FS
