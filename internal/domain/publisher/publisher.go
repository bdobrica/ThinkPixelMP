package publisher

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxSlugBytes        = 63
	MaxDisplayNameBytes = 256
	MaxDescriptionBytes = 4096
	MaxListSize         = 200
)

type State string

const (
	StateClaimed   State = "claimed"
	StateVerified  State = "verified"
	StateSuspended State = "suspended"
	StateRevoked   State = "revoked"
)

var (
	ErrInvalidTransition = errors.New("publisher: invalid state transition")
	slugPattern          = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
)

// Publisher is a tenant-local identity. Its state history is append-only.
type Publisher struct {
	tenantID    shared.UUID
	id          shared.UUID
	slug        string
	displayName string
	description string
	state       State
	version     int64
	createdAt   time.Time
}

// StateRecord describes one administrative state transition.
type StateRecord struct {
	PublisherID shared.UUID
	Version     int64
	State       State
	Reason      shared.ReasonCode
	Explanation string
	RecordedAt  time.Time
}

// Repository is the tenant-scoped persistence boundary for publishers.
// Implementations must apply TenantID to both database context and predicates,
// and join a transaction carried by the supplied context when supported.
type Repository interface {
	Create(context.Context, Publisher) error
	Get(context.Context, shared.UUID, shared.UUID) (Publisher, error)
	GetBySlug(context.Context, shared.UUID, string) (Publisher, error)
	List(context.Context, shared.UUID, *shared.UUID, int) ([]Publisher, error)
	ChangeState(context.Context, shared.UUID, shared.UUID, State, shared.ReasonCode, string, time.Time) (Publisher, error)
}

func New(tenantID, id shared.UUID, slug, displayName, description string, createdAt time.Time) (Publisher, error) {
	return restore(tenantID, id, slug, displayName, description, StateClaimed, 1, createdAt)
}

// Restore validates a Publisher loaded from an adapter.
func Restore(tenantID, id shared.UUID, slug, displayName, description string, state State, version int64, createdAt time.Time) (Publisher, error) {
	return restore(tenantID, id, slug, displayName, description, state, version, createdAt)
}

func restore(tenantID, id shared.UUID, slug, displayName, description string, state State, version int64, createdAt time.Time) (Publisher, error) {
	if _, err := tenantID.MarshalText(); err != nil {
		return Publisher{}, fmt.Errorf("publisher: tenant ID: %w", err)
	}
	if _, err := id.MarshalText(); err != nil {
		return Publisher{}, fmt.Errorf("publisher: ID: %w", err)
	}
	if err := ValidateSlug(slug); err != nil {
		return Publisher{}, fmt.Errorf("publisher: invalid slug")
	}
	if err := optionalText(displayName, MaxDisplayNameBytes); err != nil {
		return Publisher{}, fmt.Errorf("publisher: display name: %w", err)
	}
	if err := optionalText(description, MaxDescriptionBytes); err != nil {
		return Publisher{}, fmt.Errorf("publisher: description: %w", err)
	}
	if !validState(state) || version < 1 || createdAt.IsZero() {
		return Publisher{}, fmt.Errorf("publisher: invalid state, version, or creation time")
	}
	return Publisher{tenantID, id, slug, displayName, description, state, version, createdAt.UTC()}, nil
}

// ValidateSlug applies the canonical Publisher slug contract.
func ValidateSlug(value string) error {
	if len(value) > MaxSlugBytes || !slugPattern.MatchString(value) {
		return fmt.Errorf("invalid slug")
	}
	return nil
}

func optionalText(value string, maximum int) error {
	if value == "" {
		return nil
	}
	if len(value) > maximum || !utf8.ValidString(value) {
		return fmt.Errorf("invalid length or encoding")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("control character")
		}
	}
	return nil
}

func validState(state State) bool {
	switch state {
	case StateClaimed, StateVerified, StateSuspended, StateRevoked:
		return true
	default:
		return false
	}
}

func (publisher Publisher) Transition(to State, reason shared.ReasonCode, explanation string, at time.Time) (Publisher, StateRecord, error) {
	allowed := publisher.state == StateClaimed && (to == StateVerified || to == StateSuspended || to == StateRevoked) ||
		publisher.state == StateVerified && (to == StateSuspended || to == StateRevoked) ||
		publisher.state == StateSuspended && (to == StateVerified || to == StateRevoked)
	if !allowed || reason.String() == "" || at.IsZero() {
		return Publisher{}, StateRecord{}, ErrInvalidTransition
	}
	if err := optionalText(explanation, MaxDescriptionBytes); err != nil {
		return Publisher{}, StateRecord{}, fmt.Errorf("publisher: transition explanation: %w", err)
	}
	publisher.state = to
	publisher.version++
	record := StateRecord{publisher.id, publisher.version, to, reason, explanation, at.UTC()}
	return publisher, record, nil
}

func (publisher Publisher) TenantID() shared.UUID { return publisher.tenantID }
func (publisher Publisher) ID() shared.UUID       { return publisher.id }
func (publisher Publisher) Slug() string          { return publisher.slug }
func (publisher Publisher) DisplayName() string   { return publisher.displayName }
func (publisher Publisher) Description() string   { return publisher.description }
func (publisher Publisher) State() State          { return publisher.state }
func (publisher Publisher) StateVersion() int64   { return publisher.version }
func (publisher Publisher) CreatedAt() time.Time  { return publisher.createdAt }
