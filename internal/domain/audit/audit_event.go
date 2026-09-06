// Package audit defines the minimized, append-only mutation audit record.
package audit

import (
	"context"
	"fmt"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxActorBytes      = 255
	MaxResourceIDBytes = 512
	MaxReferences      = 32
	MaxListSize        = 200
)

var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Actor is verified request correlation supplied by the application boundary.
// It intentionally carries no request body or free-form metadata.
type Actor struct {
	principalID string
	requestID   *shared.UUID
	traceID     string
}

type actorContextKey struct{}

func NewActor(principalID string, requestID *shared.UUID, traceID string) (Actor, error) {
	if err := boundedSafeText(principalID, MaxActorBytes); err != nil {
		return Actor{}, fmt.Errorf("audit actor: principal ID: %w", err)
	}
	if requestID != nil {
		if _, err := requestID.MarshalText(); err != nil {
			return Actor{}, fmt.Errorf("audit actor: request ID: %w", err)
		}
		copyID := *requestID
		requestID = &copyID
	}
	if traceID != "" && !traceIDPattern.MatchString(traceID) {
		return Actor{}, fmt.Errorf("audit actor: invalid trace ID")
	}
	return Actor{principalID: principalID, requestID: requestID, traceID: traceID}, nil
}

// WithActor attaches identity that has already been derived from verified
// authentication. Callers must never populate it from tenant/body/header input.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok && actor.principalID != ""
}

func (actor Actor) PrincipalID() string { return actor.principalID }
func (actor Actor) RequestID() (shared.UUID, bool) {
	if actor.requestID == nil {
		return shared.UUID{}, false
	}
	return *actor.requestID, true
}
func (actor Actor) TraceID() string { return actor.traceID }

// Event is an immutable audit fact. Resource identifiers are opaque tenant-safe
// identifiers; arbitrary request data and free-form explanations are excluded.
type Event struct {
	tenantID       shared.UUID
	id             shared.UUID
	actorID        string
	action         shared.ReasonCode
	resourceType   shared.ReasonCode
	resourceID     string
	artifactDigest *shared.Digest
	decision       *shared.ReasonCode
	reasonCodes    []shared.ReasonCode
	evidenceIDs    []shared.UUID
	policyDigests  []shared.Digest
	occurredAt     time.Time
	requestID      *shared.UUID
	traceID        string
}

type Repository interface {
	Get(context.Context, shared.UUID, shared.UUID) (Event, error)
	List(context.Context, shared.UUID, *shared.UUID, int) ([]Event, error)
}

func Restore(tenantID, id shared.UUID, actorID string, action, resourceType shared.ReasonCode, resourceID string,
	artifactDigest *shared.Digest, decision *shared.ReasonCode, reasonCodes []shared.ReasonCode,
	evidenceIDs []shared.UUID, policyDigests []shared.Digest, occurredAt time.Time, requestID *shared.UUID, traceID string,
) (Event, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "event ID": id} {
		if _, err := identifier.MarshalText(); err != nil {
			return Event{}, fmt.Errorf("audit event: %s: %w", label, err)
		}
	}
	if err := boundedSafeText(actorID, MaxActorBytes); err != nil {
		return Event{}, fmt.Errorf("audit event: actor ID: %w", err)
	}
	if action.String() == "" || resourceType.String() == "" {
		return Event{}, fmt.Errorf("audit event: action and resource type are required")
	}
	if err := boundedSafeText(resourceID, MaxResourceIDBytes); err != nil {
		return Event{}, fmt.Errorf("audit event: resource ID: %w", err)
	}
	if artifactDigest != nil {
		if _, err := artifactDigest.MarshalText(); err != nil {
			return Event{}, fmt.Errorf("audit event: artifact digest: %w", err)
		}
	}
	if decision != nil && decision.String() == "" {
		return Event{}, fmt.Errorf("audit event: invalid decision")
	}
	if len(reasonCodes) > MaxReferences || len(evidenceIDs) > MaxReferences || len(policyDigests) > MaxReferences {
		return Event{}, fmt.Errorf("audit event: too many references")
	}
	for _, code := range reasonCodes {
		if code.String() == "" {
			return Event{}, fmt.Errorf("audit event: invalid reason code")
		}
	}
	for _, identifier := range evidenceIDs {
		if _, err := identifier.MarshalText(); err != nil {
			return Event{}, fmt.Errorf("audit event: evidence ID: %w", err)
		}
	}
	for _, digest := range policyDigests {
		if _, err := digest.MarshalText(); err != nil {
			return Event{}, fmt.Errorf("audit event: policy digest: %w", err)
		}
	}
	if occurredAt.IsZero() {
		return Event{}, fmt.Errorf("audit event: occurrence time required")
	}
	if requestID != nil {
		if _, err := requestID.MarshalText(); err != nil {
			return Event{}, fmt.Errorf("audit event: request ID: %w", err)
		}
	}
	if traceID != "" && !traceIDPattern.MatchString(traceID) {
		return Event{}, fmt.Errorf("audit event: invalid trace ID")
	}
	return Event{tenantID, id, actorID, action, resourceType, resourceID, copyDigest(artifactDigest), copyReason(decision),
		append([]shared.ReasonCode(nil), reasonCodes...), append([]shared.UUID(nil), evidenceIDs...), append([]shared.Digest(nil), policyDigests...),
		occurredAt.UTC(), copyUUID(requestID), traceID}, nil
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

func copyUUID(value *shared.UUID) *shared.UUID {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
func copyDigest(value *shared.Digest) *shared.Digest {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
func copyReason(value *shared.ReasonCode) *shared.ReasonCode {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func (event Event) TenantID() shared.UUID           { return event.tenantID }
func (event Event) ID() shared.UUID                 { return event.id }
func (event Event) ActorID() string                 { return event.actorID }
func (event Event) Action() shared.ReasonCode       { return event.action }
func (event Event) ResourceType() shared.ReasonCode { return event.resourceType }
func (event Event) ResourceID() string              { return event.resourceID }
func (event Event) ArtifactDigest() (shared.Digest, bool) {
	if event.artifactDigest == nil {
		return shared.Digest{}, false
	}
	return *event.artifactDigest, true
}
func (event Event) Decision() (shared.ReasonCode, bool) {
	if event.decision == nil {
		return shared.ReasonCode{}, false
	}
	return *event.decision, true
}
func (event Event) ReasonCodes() []shared.ReasonCode {
	return append([]shared.ReasonCode(nil), event.reasonCodes...)
}
func (event Event) EvidenceIDs() []shared.UUID {
	return append([]shared.UUID(nil), event.evidenceIDs...)
}
func (event Event) PolicyDigests() []shared.Digest {
	return append([]shared.Digest(nil), event.policyDigests...)
}
func (event Event) OccurredAt() time.Time { return event.occurredAt }
func (event Event) RequestID() (shared.UUID, bool) {
	if event.requestID == nil {
		return shared.UUID{}, false
	}
	return *event.requestID, true
}
func (event Event) TraceID() string { return event.traceID }
