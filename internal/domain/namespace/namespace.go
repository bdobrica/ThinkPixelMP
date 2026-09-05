// Package namespace models tenant-local hierarchical artifact namespaces.
package namespace

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxPathBytes = 255
	MaxListSize  = 200
)

var pathPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)

// Namespace is an immutable tenant-local path and its original owning Publisher.
// Ownership permits publication only; it grants no runtime authority.
type Namespace struct {
	tenantID         shared.UUID
	id               shared.UUID
	path             string
	ownerPublisherID shared.UUID
	createdAt        time.Time
}

// Repository is the tenant-scoped persistence boundary for namespaces.
// Implementations must apply TenantID to both database context and predicates.
type Repository interface {
	Create(context.Context, Namespace) error
	Get(context.Context, shared.UUID, shared.UUID) (Namespace, error)
	GetByPath(context.Context, shared.UUID, string) (Namespace, error)
	List(context.Context, shared.UUID, *shared.UUID, int) ([]Namespace, error)
}

func New(tenantID, id shared.UUID, path string, ownerPublisherID shared.UUID, createdAt time.Time) (Namespace, error) {
	if _, err := tenantID.MarshalText(); err != nil {
		return Namespace{}, fmt.Errorf("namespace: tenant ID: %w", err)
	}
	if _, err := id.MarshalText(); err != nil {
		return Namespace{}, fmt.Errorf("namespace: ID: %w", err)
	}
	if _, err := ownerPublisherID.MarshalText(); err != nil {
		return Namespace{}, fmt.Errorf("namespace: owner Publisher ID: %w", err)
	}
	if err := ValidatePath(path); err != nil {
		return Namespace{}, fmt.Errorf("namespace: invalid path")
	}
	if createdAt.IsZero() {
		return Namespace{}, fmt.Errorf("namespace: creation time is required")
	}
	return Namespace{tenantID, id, path, ownerPublisherID, createdAt.UTC()}, nil
}

// Restore validates a Namespace loaded from an adapter.
func Restore(tenantID, id shared.UUID, path string, ownerPublisherID shared.UUID, createdAt time.Time) (Namespace, error) {
	return New(tenantID, id, path, ownerPublisherID, createdAt)
}

// ValidatePath applies the canonical V1 Namespace path contract.
func ValidatePath(value string) error {
	if len(value) > MaxPathBytes || !pathPattern.MatchString(value) {
		return fmt.Errorf("invalid path")
	}
	return nil
}

func (namespace Namespace) TenantID() shared.UUID         { return namespace.tenantID }
func (namespace Namespace) ID() shared.UUID               { return namespace.id }
func (namespace Namespace) Path() string                  { return namespace.path }
func (namespace Namespace) OwnerPublisherID() shared.UUID { return namespace.ownerPublisherID }
func (namespace Namespace) CreatedAt() time.Time          { return namespace.createdAt }
