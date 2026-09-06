// Package outbox defines immutable marketplace events and bounded delivery state.
package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	DataContentType     = "application/vnd.thinkpixel.marketplace-event.v1+json"
	MaxPayloadBytes     = 256 * 1024
	MaxClaimBatch       = 100
	MaxAttempts         = 1000
	MaxLease            = 15 * time.Minute
	MaxRetryDelay       = 24 * time.Hour
	DeliveredRetention  = 30 * 24 * time.Hour
	DeadLetterRetention = 90 * 24 * time.Hour
	maxSourceBytes      = 1024
	maxSubjectBytes     = 512
	maxWorkerBytes      = 255
)

type State string

const (
	StatePending    State = "pending"
	StateClaimed    State = "claimed"
	StateRetry      State = "retry"
	StateDelivered  State = "delivered"
	StateDeadLetter State = "dead_letter"
)

// Message preserves the exact serialized CloudEvent bytes across every retry.
// Delivery metadata is bounded and contains only stable reason codes.
type Message struct {
	tenantID       shared.UUID
	id             shared.UUID
	sequence       uint64
	source         string
	eventType      shared.ReasonCode
	subject        string
	payload        []byte
	payloadDigest  shared.Digest
	state          State
	attempts       uint16
	availableAt    time.Time
	claimToken     *shared.UUID
	claimedBy      string
	claimedAt      *time.Time
	leaseExpiresAt *time.Time
	lastError      *shared.ReasonCode
	deliveredAt    *time.Time
	deadLetteredAt *time.Time
	retainUntil    *time.Time
	createdAt      time.Time
}

// Repository is the tenant-scoped delivery boundary for outbox messages. Its
// operations join a transaction carried by context when supported.
type Repository interface {
	Get(context.Context, shared.UUID, shared.UUID) (Message, error)
	Claim(context.Context, shared.UUID, string, shared.UUID, time.Time, time.Duration, int) ([]Message, error)
	Retry(context.Context, shared.UUID, shared.UUID, shared.UUID, shared.ReasonCode, time.Time) (Message, error)
	Deliver(context.Context, shared.UUID, shared.UUID, shared.UUID, time.Time) (Message, error)
	DeadLetter(context.Context, shared.UUID, shared.UUID, shared.UUID, shared.ReasonCode, time.Time) (Message, error)
}

// Writer appends an immutable message using a sequence allocated in the same
// application transaction. Calls outside a transaction must fail closed.
type Writer interface {
	NextSequence(context.Context, shared.UUID) (uint64, error)
	Record(context.Context, Message) error
}

type envelope struct {
	SpecVersion     string                     `json:"specversion"`
	ID              string                     `json:"id"`
	Source          string                     `json:"source"`
	Type            string                     `json:"type"`
	Subject         string                     `json:"subject"`
	Time            time.Time                  `json:"time"`
	DataContentType string                     `json:"datacontenttype"`
	Sequence        uint64                     `json:"sequence"`
	Data            map[string]json.RawMessage `json:"data"`
}

func New(tenantID, id shared.UUID, sequence uint64, source, eventType, subject string, payload []byte, createdAt time.Time) (Message, error) {
	return Restore(tenantID, id, sequence, source, eventType, subject, payload, shared.SHA256Digest(payload),
		StatePending, 0, createdAt, nil, "", nil, nil, nil, nil, nil, nil, createdAt)
}

func Restore(tenantID, id shared.UUID, sequence uint64, source, eventType, subject string, payload []byte,
	payloadDigest shared.Digest, state State, attempts uint16, availableAt time.Time, claimToken *shared.UUID,
	claimedBy string, claimedAt, leaseExpiresAt *time.Time, lastError *shared.ReasonCode,
	deliveredAt, deadLetteredAt, retainUntil *time.Time, createdAt time.Time,
) (Message, error) {
	for label, value := range map[string]shared.UUID{"tenant ID": tenantID, "message ID": id} {
		if _, err := value.MarshalText(); err != nil {
			return Message{}, fmt.Errorf("outbox message: %s: %w", label, err)
		}
	}
	if sequence == 0 || sequence > uint64(^uint64(0)>>1) {
		return Message{}, fmt.Errorf("outbox message: invalid sequence")
	}
	if err := validateText(source, maxSourceBytes); err != nil {
		return Message{}, fmt.Errorf("outbox message: source: %w", err)
	}
	parsedSource, err := url.Parse(source)
	if err != nil || parsedSource.String() != source || parsedSource.Scheme != "urn" || parsedSource.RawQuery != "" || parsedSource.User != nil {
		return Message{}, fmt.Errorf("outbox message: invalid source")
	}
	typeCode, err := shared.NewReasonCode(eventType)
	if err != nil || !strings.HasPrefix(eventType, "io.thinkpixel.mp.") || !strings.HasSuffix(eventType, ".v1") {
		return Message{}, fmt.Errorf("outbox message: invalid event type")
	}
	if err := validateText(subject, maxSubjectBytes); err != nil {
		return Message{}, fmt.Errorf("outbox message: subject: %w", err)
	}
	if len(payload) < 2 || len(payload) > MaxPayloadBytes || !json.Valid(payload) {
		return Message{}, fmt.Errorf("outbox message: invalid payload")
	}
	if shared.SHA256Digest(payload) != payloadDigest {
		return Message{}, fmt.Errorf("outbox message: payload digest mismatch")
	}
	if err := validateEnvelope(payload, tenantID, id, sequence, source, eventType, subject, createdAt); err != nil {
		return Message{}, err
	}
	if createdAt.IsZero() || availableAt.Before(createdAt) {
		return Message{}, fmt.Errorf("outbox message: invalid availability")
	}
	if attempts > MaxAttempts {
		return Message{}, fmt.Errorf("outbox message: too many attempts")
	}
	if err := validateDelivery(state, attempts, availableAt, claimToken, claimedBy, claimedAt, leaseExpiresAt, lastError, deliveredAt, deadLetteredAt, retainUntil); err != nil {
		return Message{}, err
	}
	return Message{tenantID, id, sequence, source, typeCode, subject, bytes.Clone(payload), payloadDigest, state, attempts,
		availableAt.UTC(), copyUUID(claimToken), claimedBy, copyTime(claimedAt), copyTime(leaseExpiresAt), copyReason(lastError),
		copyTime(deliveredAt), copyTime(deadLetteredAt), copyTime(retainUntil), createdAt.UTC()}, nil
}

func validateEnvelope(payload []byte, tenantID, id shared.UUID, sequence uint64, source, eventType, subject string, createdAt time.Time) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var value envelope
	if err := decoder.Decode(&value); err != nil || value.SpecVersion != "1.0" || value.ID != id.String() || value.Source != source ||
		value.Type != eventType || value.Subject != subject || value.DataContentType != DataContentType || value.Sequence != sequence || !value.Time.Equal(createdAt) {
		return fmt.Errorf("outbox message: payload does not match envelope metadata")
	}
	tenant, ok := value.Data["tenant_id"]
	var tenantValue string
	if !ok || json.Unmarshal(tenant, &tenantValue) != nil || tenantValue != tenantID.String() {
		return fmt.Errorf("outbox message: payload tenant mismatch")
	}
	cursor, ok := value.Data["transaction_cursor"]
	var cursorValue string
	if !ok || json.Unmarshal(cursor, &cursorValue) != nil || cursorValue == "" || len(cursorValue) > 1024 {
		return fmt.Errorf("outbox message: transaction cursor required")
	}
	if err := validateEventData(eventType, value.Data); err != nil {
		return fmt.Errorf("outbox message: event data: %w", err)
	}
	return nil
}

type dataSpec struct {
	required []string
	optional []string
	uuids    []string
	digests  []string
	reasons  []string
	enums    map[string][]string
}

var eventDataSpecs = map[string]dataSpec{
	"io.thinkpixel.mp.artifact.registered.v1": {
		required: []string{"tenant_id", "transaction_cursor", "artifact_version_id", "artifact_digest", "descriptor_digest"},
		uuids:    []string{"tenant_id", "artifact_version_id"}, digests: []string{"artifact_digest", "descriptor_digest"},
	},
	"io.thinkpixel.mp.artifact.deprecated.v1":          artifactLifecycleSpec("deprecated"),
	"io.thinkpixel.mp.artifact.quarantined.v1":         artifactLifecycleSpec("quarantined"),
	"io.thinkpixel.mp.artifact.quarantine-released.v1": artifactLifecycleSpec("active"),
	"io.thinkpixel.mp.artifact.revoked.v1":             artifactLifecycleSpec("revoked"),
	"io.thinkpixel.mp.evidence.accepted.v1": {
		required: []string{"tenant_id", "transaction_cursor", "evidence_record_id", "artifact_digest", "category", "conclusion", "report_digest"},
		uuids:    []string{"tenant_id", "evidence_record_id"}, digests: []string{"artifact_digest", "report_digest"}, reasons: []string{"category"},
		enums: map[string][]string{"conclusion": {"pass", "fail", "warning", "unknown", "not-applicable"}},
	},
	"io.thinkpixel.mp.policy.activated.v1": {
		required: []string{"tenant_id", "transaction_cursor", "catalog_id", "policy_digest", "activation_digest"},
		uuids:    []string{"tenant_id", "catalog_id"}, digests: []string{"policy_digest", "activation_digest"},
	},
	"io.thinkpixel.mp.promotion.decided.v1": {
		required: []string{"tenant_id", "transaction_cursor", "promotion_request_id", "promotion_decision_id", "artifact_digest", "catalog_id", "decision"},
		uuids:    []string{"tenant_id", "promotion_request_id", "promotion_decision_id", "catalog_id"}, digests: []string{"artifact_digest"},
		enums: map[string][]string{"decision": {"approved", "denied", "expired", "cancelled"}},
	},
	"io.thinkpixel.mp.catalog-entry.changed.v1": {
		required: []string{"tenant_id", "transaction_cursor", "catalog_id", "catalog_entry_id", "artifact_digest", "previous_state", "current_state"},
		uuids:    []string{"tenant_id", "catalog_id", "catalog_entry_id"}, digests: []string{"artifact_digest"},
		enums: map[string][]string{"previous_state": {"absent", "active", "removed"}, "current_state": {"active", "removed"}},
	},
	"io.thinkpixel.mp.resolution.created.v1": {
		required: []string{"tenant_id", "transaction_cursor", "resolution_id", "resolution_digest", "root_digest", "lock_digest"},
		uuids:    []string{"tenant_id", "resolution_id"}, digests: []string{"resolution_digest", "root_digest", "lock_digest"},
	},
	"io.thinkpixel.mp.import.completed.v1":  importFinishedSpec("completed"),
	"io.thinkpixel.mp.import.failed.v1":     importFinishedSpec("failed"),
	"io.thinkpixel.mp.import.conflicted.v1": importFinishedSpec("conflicted"),
}

func artifactLifecycleSpec(current string) dataSpec {
	return dataSpec{
		required: []string{"tenant_id", "transaction_cursor", "artifact_version_id", "artifact_digest", "previous_state", "current_state", "reason_code"},
		uuids:    []string{"tenant_id", "artifact_version_id"}, digests: []string{"artifact_digest"}, reasons: []string{"reason_code"},
		enums: map[string][]string{"previous_state": {"active", "deprecated", "quarantined"}, "current_state": {current}},
	}
}

func importFinishedSpec(outcome string) dataSpec {
	return dataSpec{
		required: []string{"tenant_id", "transaction_cursor", "import_source_id", "import_run_id", "outcome"}, optional: []string{"reason_code"},
		uuids: []string{"tenant_id", "import_source_id", "import_run_id"}, reasons: []string{"reason_code"},
		enums: map[string][]string{"outcome": {outcome}},
	}
}

func validateEventData(eventType string, data map[string]json.RawMessage) error {
	spec, ok := eventDataSpecs[eventType]
	if !ok {
		return fmt.Errorf("unsupported event type")
	}
	allowed := make(map[string]bool, len(spec.required)+len(spec.optional))
	for _, key := range append(append([]string(nil), spec.required...), spec.optional...) {
		allowed[key] = true
	}
	for key := range data {
		if !allowed[key] {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	for _, key := range spec.required {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("missing field %q", key)
		}
	}
	for _, key := range spec.uuids {
		if raw, ok := data[key]; ok {
			value, err := rawString(raw)
			if err != nil {
				return err
			}
			if _, err := shared.ParseUUID(value); err != nil {
				return fmt.Errorf("invalid field %q", key)
			}
		}
	}
	for _, key := range spec.digests {
		value, err := rawString(data[key])
		if err != nil {
			return err
		}
		if _, err := shared.ParseDigest(value); err != nil {
			return fmt.Errorf("invalid field %q", key)
		}
	}
	for _, key := range spec.reasons {
		if raw, ok := data[key]; ok {
			value, err := rawString(raw)
			if err != nil {
				return err
			}
			if _, err := shared.NewReasonCode(value); err != nil {
				return fmt.Errorf("invalid field %q", key)
			}
		}
	}
	for key, values := range spec.enums {
		value, err := rawString(data[key])
		if err != nil {
			return err
		}
		valid := false
		for _, candidate := range values {
			valid = valid || value == candidate
		}
		if !valid {
			return fmt.Errorf("invalid field %q", key)
		}
	}
	return nil
}

func rawString(raw json.RawMessage) (string, error) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("event data field must be a string")
	}
	return value, nil
}

func validateDelivery(state State, attempts uint16, availableAt time.Time, token *shared.UUID, worker string,
	claimedAt, leaseUntil *time.Time, lastError *shared.ReasonCode, deliveredAt, deadAt, retainUntil *time.Time,
) error {
	claimed := token != nil || worker != "" || claimedAt != nil || leaseUntil != nil
	switch state {
	case StatePending, StateRetry:
		if claimed || deliveredAt != nil || deadAt != nil || retainUntil != nil ||
			state == StatePending && (attempts != 0 || lastError != nil) ||
			state == StateRetry && (attempts == 0 || lastError == nil) {
			return fmt.Errorf("outbox message: invalid queued state")
		}
	case StateClaimed:
		if token == nil || attempts == 0 || validateText(worker, maxWorkerBytes) != nil || claimedAt == nil || leaseUntil == nil ||
			!leaseUntil.After(*claimedAt) || leaseUntil.After(claimedAt.Add(MaxLease)) || deliveredAt != nil || deadAt != nil || retainUntil != nil {
			return fmt.Errorf("outbox message: invalid claim")
		}
		if _, err := token.MarshalText(); err != nil {
			return fmt.Errorf("outbox message: invalid claim token")
		}
	case StateDelivered:
		if claimed || deliveredAt == nil || deadAt != nil || retainUntil == nil || retainUntil.Before(deliveredAt.Add(DeliveredRetention)) {
			return fmt.Errorf("outbox message: invalid delivery")
		}
	case StateDeadLetter:
		if claimed || deadAt == nil || lastError == nil || lastError.String() == "" || deliveredAt != nil || retainUntil == nil || retainUntil.Before(deadAt.Add(DeadLetterRetention)) {
			return fmt.Errorf("outbox message: invalid dead letter")
		}
	default:
		return fmt.Errorf("outbox message: invalid state")
	}
	return nil
}

func validateText(value string, maximum int) error {
	_, err := shared.NewBoundedString(value, maximum)
	return err
}
func copyUUID(value *shared.UUID) *shared.UUID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := value.UTC()
	return &copied
}
func copyReason(value *shared.ReasonCode) *shared.ReasonCode {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func (m Message) TenantID() shared.UUID        { return m.tenantID }
func (m Message) ID() shared.UUID              { return m.id }
func (m Message) Sequence() uint64             { return m.sequence }
func (m Message) Source() string               { return m.source }
func (m Message) EventType() shared.ReasonCode { return m.eventType }
func (m Message) Subject() string              { return m.subject }
func (m Message) Payload() []byte              { return bytes.Clone(m.payload) }
func (m Message) PayloadDigest() shared.Digest { return m.payloadDigest }
func (m Message) State() State                 { return m.state }
func (m Message) Attempts() uint16             { return m.attempts }
func (m Message) AvailableAt() time.Time       { return m.availableAt }
func (m Message) ClaimToken() (shared.UUID, bool) {
	if m.claimToken == nil {
		return shared.UUID{}, false
	}
	return *m.claimToken, true
}
func (m Message) ClaimedBy() string { return m.claimedBy }
func (m Message) ClaimedAt() (time.Time, bool) {
	if m.claimedAt == nil {
		return time.Time{}, false
	}
	return *m.claimedAt, true
}
func (m Message) LeaseExpiresAt() (time.Time, bool) {
	if m.leaseExpiresAt == nil {
		return time.Time{}, false
	}
	return *m.leaseExpiresAt, true
}
func (m Message) LastError() (shared.ReasonCode, bool) {
	if m.lastError == nil {
		return shared.ReasonCode{}, false
	}
	return *m.lastError, true
}
func (m Message) DeliveredAt() (time.Time, bool) {
	if m.deliveredAt == nil {
		return time.Time{}, false
	}
	return *m.deliveredAt, true
}
func (m Message) DeadLetteredAt() (time.Time, bool) {
	if m.deadLetteredAt == nil {
		return time.Time{}, false
	}
	return *m.deadLetteredAt, true
}
func (m Message) RetainUntil() (time.Time, bool) {
	if m.retainUntil == nil {
		return time.Time{}, false
	}
	return *m.retainUntil, true
}
func (m Message) CreatedAt() time.Time { return m.createdAt }
