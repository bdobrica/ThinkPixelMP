// Package idempotency defines request ownership and established mutation results.
package idempotency

import (
	"context"
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxPrincipalBytes  = 255
	MaxKeyBytes        = 255
	MaxResourceIDBytes = 512
	MinimumRetention   = 24 * time.Hour
)

type State string

const (
	StatePending   State = "pending"
	StateCompleted State = "completed"
)

// Result is the stable response established by a completed mutation. Resource
// identity is optional, but its type and identifier are always present together.
type Result struct {
	status       uint16
	resourceType shared.ReasonCode
	resourceID   string
}

func NewResult(status uint16, resourceType, resourceID string) (Result, error) {
	if status < 100 || status > 599 {
		return Result{}, fmt.Errorf("idempotency result: invalid response status")
	}
	if (resourceType == "") != (resourceID == "") {
		return Result{}, fmt.Errorf("idempotency result: incomplete resource identity")
	}
	var parsedType shared.ReasonCode
	if resourceType != "" {
		var err error
		parsedType, err = shared.NewReasonCode(resourceType)
		if err != nil {
			return Result{}, fmt.Errorf("idempotency result: resource type: %w", err)
		}
		if err := boundedSafeText(resourceID, MaxResourceIDBytes); err != nil {
			return Result{}, fmt.Errorf("idempotency result: resource ID: %w", err)
		}
	}
	return Result{status: status, resourceType: parsedType, resourceID: resourceID}, nil
}

func (result Result) Status() uint16 { return result.status }
func (result Result) ResourceType() (shared.ReasonCode, bool) {
	return result.resourceType, result.resourceType.String() != ""
}
func (result Result) ResourceID() (string, bool) { return result.resourceID, result.resourceID != "" }

// Record binds one caller-owned key to the digest of one canonical request.
// Only its pending-to-completed result transition is mutable.
type Record struct {
	tenantID    shared.UUID
	id          shared.UUID
	principal   string
	action      shared.ReasonCode
	key         string
	digest      shared.Digest
	state       State
	result      *Result
	createdAt   time.Time
	completedAt *time.Time
	expiresAt   time.Time
}

// Repository is the tenant-scoped idempotency persistence boundary. Its
// operations join a transaction carried by context when supported.
type Repository interface {
	Acquire(context.Context, Record) (Record, bool, error)
	Get(context.Context, shared.UUID, string, shared.ReasonCode, string) (Record, error)
	Complete(context.Context, shared.UUID, string, shared.ReasonCode, string, shared.Digest, Result, time.Time) (Record, error)
}

func New(tenantID, id shared.UUID, principal string, action shared.ReasonCode, key string,
	digest shared.Digest, createdAt, expiresAt time.Time,
) (Record, error) {
	return restore(tenantID, id, principal, action, key, digest, StatePending, nil, createdAt, nil, expiresAt)
}

// ValidateOwnership validates values derived from authenticated identity and the
// request boundary without requiring a full record to be constructed.
func ValidateOwnership(principal string, action shared.ReasonCode, key string) error {
	if err := boundedSafeText(principal, MaxPrincipalBytes); err != nil {
		return fmt.Errorf("principal: %w", err)
	}
	if action.String() == "" {
		return fmt.Errorf("action required")
	}
	if err := boundedSafeText(key, MaxKeyBytes); err != nil {
		return fmt.Errorf("key: %w", err)
	}
	return nil
}

func Restore(tenantID, id shared.UUID, principal string, action shared.ReasonCode, key string,
	digest shared.Digest, state State, result *Result, createdAt time.Time, completedAt *time.Time, expiresAt time.Time,
) (Record, error) {
	return restore(tenantID, id, principal, action, key, digest, state, result, createdAt, completedAt, expiresAt)
}

func restore(tenantID, id shared.UUID, principal string, action shared.ReasonCode, key string,
	digest shared.Digest, state State, result *Result, createdAt time.Time, completedAt *time.Time, expiresAt time.Time,
) (Record, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "record ID": id} {
		if _, err := identifier.MarshalText(); err != nil {
			return Record{}, fmt.Errorf("idempotency record: %s: %w", label, err)
		}
	}
	if err := ValidateOwnership(principal, action, key); err != nil {
		return Record{}, fmt.Errorf("idempotency record: %w", err)
	}
	if _, err := digest.MarshalText(); err != nil {
		return Record{}, fmt.Errorf("idempotency record: request digest: %w", err)
	}
	if createdAt.IsZero() || expiresAt.Before(createdAt.Add(MinimumRetention)) {
		return Record{}, fmt.Errorf("idempotency record: invalid retention window")
	}
	createdAt = createdAt.UTC()
	expiresAt = expiresAt.UTC()
	switch state {
	case StatePending:
		if result != nil || completedAt != nil {
			return Record{}, fmt.Errorf("idempotency record: pending record has a result")
		}
	case StateCompleted:
		if result == nil || result.status < 100 || completedAt == nil || completedAt.Before(createdAt) {
			return Record{}, fmt.Errorf("idempotency record: completed record requires a valid result")
		}
		copyTime := completedAt.UTC()
		completedAt = &copyTime
	default:
		return Record{}, fmt.Errorf("idempotency record: invalid state")
	}
	return Record{tenantID: tenantID, id: id, principal: principal, action: action, key: key, digest: digest,
		state: state, result: copyResult(result), createdAt: createdAt, completedAt: completedAt, expiresAt: expiresAt}, nil
}

func boundedSafeText(value string, maximum int) error {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return fmt.Errorf("invalid length or encoding")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("control character")
		}
	}
	return nil
}

func copyResult(value *Result) *Result {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func (record Record) TenantID() shared.UUID        { return record.tenantID }
func (record Record) ID() shared.UUID              { return record.id }
func (record Record) Principal() string            { return record.principal }
func (record Record) Action() shared.ReasonCode    { return record.action }
func (record Record) Key() string                  { return record.key }
func (record Record) RequestDigest() shared.Digest { return record.digest }
func (record Record) State() State                 { return record.state }
func (record Record) Result() (Result, bool) {
	if record.result == nil {
		return Result{}, false
	}
	return *record.result, true
}
func (record Record) CreatedAt() time.Time { return record.createdAt }
func (record Record) CompletedAt() (time.Time, bool) {
	if record.completedAt == nil {
		return time.Time{}, false
	}
	return *record.completedAt, true
}
func (record Record) ExpiresAt() time.Time { return record.expiresAt }
