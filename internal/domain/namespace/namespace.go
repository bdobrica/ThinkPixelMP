// Package namespace models tenant-local hierarchical artifact namespaces.
package namespace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxPathBytes = 255
	MaxListSize  = 200
)

var pathPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)

var ErrInvalidDelegationTransition = errors.New("namespace: invalid delegation transition")

type DelegationState string

const (
	DelegationActive  DelegationState = "active"
	DelegationRevoked DelegationState = "revoked"
)

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
// Implementations must apply TenantID to database context and predicates, join
// a transaction carried by context when supported, and reject Create when the
// owner is not a verified Publisher in the same tenant.
type Repository interface {
	Create(context.Context, Namespace) error
	Get(context.Context, shared.UUID, shared.UUID) (Namespace, error)
	GetByPath(context.Context, shared.UUID, string) (Namespace, error)
	List(context.Context, shared.UUID, *shared.UUID, int) ([]Namespace, error)
}

// DelegationRepository persists append-only namespace delegation history and
// resolves the single longest valid publication prefix within a tenant.
type DelegationRepository interface {
	CreateDelegation(context.Context, Delegation) error
	GetDelegation(context.Context, shared.UUID, shared.UUID) (Delegation, error)
	RevokeDelegation(context.Context, shared.UUID, shared.UUID, int64, shared.ReasonCode, string, time.Time) (Delegation, error)
	ResolveOwner(context.Context, shared.UUID, string) (shared.UUID, error)
}

// Delegation grants a verified Publisher control of a strict child prefix. Its
// state history is append-only; it grants publication authority only.
type Delegation struct {
	tenantID, id, namespaceID, publisherID shared.UUID
	childPrefix                            string
	state                                  DelegationState
	version                                int64
	createdAt                              time.Time
}

func NewDelegation(tenantID, id, namespaceID shared.UUID, namespacePath string, publisherID shared.UUID, childPrefix string, createdAt time.Time) (Delegation, error) {
	return restoreDelegation(tenantID, id, namespaceID, publisherID, namespacePath, childPrefix, DelegationActive, 1, createdAt)
}

func RestoreDelegation(tenantID, id, namespaceID, publisherID shared.UUID, childPrefix string, state DelegationState, version int64, createdAt time.Time) (Delegation, error) {
	return restoreDelegation(tenantID, id, namespaceID, publisherID, "", childPrefix, state, version, createdAt)
}

func restoreDelegation(tenantID, id, namespaceID, publisherID shared.UUID, namespacePath, childPrefix string, state DelegationState, version int64, createdAt time.Time) (Delegation, error) {
	for _, value := range []shared.UUID{tenantID, id, namespaceID, publisherID} {
		if _, err := value.MarshalText(); err != nil {
			return Delegation{}, fmt.Errorf("namespace: delegation identifier required")
		}
	}
	if err := ValidatePath(childPrefix); err != nil || namespacePath != "" && !IsStrictDescendant(namespacePath, childPrefix) {
		return Delegation{}, fmt.Errorf("namespace: invalid delegation prefix")
	}
	if state == DelegationActive && version != 1 || state == DelegationRevoked && version != 2 ||
		state != DelegationActive && state != DelegationRevoked || createdAt.IsZero() {
		return Delegation{}, fmt.Errorf("namespace: invalid delegation state")
	}
	return Delegation{tenantID, id, namespaceID, publisherID, childPrefix, state, version, createdAt.UTC()}, nil
}

func IsStrictDescendant(parent, candidate string) bool {
	return ValidatePath(parent) == nil && ValidatePath(candidate) == nil && strings.HasPrefix(candidate, parent+"/")
}

func (delegation Delegation) Revoke(reason shared.ReasonCode, explanation string, at time.Time) (Delegation, error) {
	if delegation.state != DelegationActive || reason.String() == "" || at.IsZero() || at.Before(delegation.createdAt) || len(explanation) > 4096 || !utf8.ValidString(explanation) {
		return Delegation{}, ErrInvalidDelegationTransition
	}
	for _, character := range explanation {
		if unicode.IsControl(character) {
			return Delegation{}, ErrInvalidDelegationTransition
		}
	}
	delegation.state = DelegationRevoked
	delegation.version++
	return delegation, nil
}

func (delegation Delegation) TenantID() shared.UUID    { return delegation.tenantID }
func (delegation Delegation) ID() shared.UUID          { return delegation.id }
func (delegation Delegation) NamespaceID() shared.UUID { return delegation.namespaceID }
func (delegation Delegation) PublisherID() shared.UUID { return delegation.publisherID }
func (delegation Delegation) ChildPrefix() string      { return delegation.childPrefix }
func (delegation Delegation) State() DelegationState   { return delegation.state }
func (delegation Delegation) StateVersion() int64      { return delegation.version }
func (delegation Delegation) CreatedAt() time.Time     { return delegation.createdAt }

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
