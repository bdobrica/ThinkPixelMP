// Package transaction defines application transaction composition without
// exposing a database driver or adapter transaction type.
package transaction

import (
	"context"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

// Manager executes work atomically within one authenticated tenant scope.
// Repository operations called with the callback context join the transaction.
// Returning an error rolls back all work and preserves that error.
type Manager interface {
	WithinTransaction(context.Context, shared.UUID, func(context.Context) error) error
}
