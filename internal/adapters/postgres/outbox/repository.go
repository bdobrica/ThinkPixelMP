// Package outbox persists and claims tenant-scoped transactional outbox messages.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/outbox"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct{ db beginner }

func NewRepository(db beginner) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("outbox repository: database is required")
	}
	return &Repository{db: db}, nil
}

// NextSequence allocates a tenant-local event sequence through the caller's
// transaction. Rolling that transaction back also rolls the allocation back.
func NextSequence(ctx context.Context, tx pgx.Tx, tenantID shared.UUID) (uint64, error) {
	if tx == nil || !validUUID(tenantID) {
		return 0, typed(shared.ErrorInvalid, "outbox.invalid_sequence_request")
	}
	var sequence int64
	err := tx.QueryRow(ctx, `INSERT INTO public.tenant_event_sequences (tenant_id, next_sequence)
 VALUES ($1::uuid, 2)
 ON CONFLICT (tenant_id) DO UPDATE
 SET next_sequence = public.tenant_event_sequences.next_sequence + 1
 WHERE public.tenant_event_sequences.next_sequence < 9223372036854775807
 RETURNING next_sequence - 1`, tenantID.String()).Scan(&sequence)
	if err != nil || sequence < 1 {
		return 0, unavailable()
	}
	return uint64(sequence), nil
}

// Record inserts immutable event bytes through the caller's transaction. It
// never commits independently, so domain, audit, and outbox writes can be atomic.
func Record(ctx context.Context, tx pgx.Tx, value domain.Message) error {
	if tx == nil || !validUUID(value.TenantID()) || value.Sequence() == 0 || value.State() != domain.StatePending || value.Attempts() != 0 {
		return typed(shared.ErrorInvalid, "outbox.invalid_message")
	}
	result, err := tx.Exec(ctx, `INSERT INTO public.outbox_messages
  (tenant_id, outbox_message_id, sequence, event_source, event_type, event_subject,
   event_payload, payload_digest, available_at, created_at)
 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $9)`,
		value.TenantID().String(), value.ID().String(), value.Sequence(), value.Source(), value.EventType().String(),
		value.Subject(), value.Payload(), value.PayloadDigest().String(), value.CreatedAt())
	if err != nil || result.RowsAffected() != 1 {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, messageID shared.UUID) (domain.Message, error) {
	if !validUUID(messageID) {
		return domain.Message{}, typed(shared.ErrorInvalid, "outbox.invalid_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Message{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, selectMessage+` WHERE tenant_id = $1::uuid AND outbox_message_id = $2::uuid`, tenantID.String(), messageID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, typed(shared.ErrorNotFound, "outbox.not_found")
	}
	if err != nil {
		return domain.Message{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Message{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) Claim(ctx context.Context, tenantID shared.UUID, worker string, claimToken shared.UUID,
	claimedAt time.Time, lease time.Duration, limit int,
) ([]domain.Message, error) {
	if !validUUID(claimToken) || claimedAt.IsZero() || lease <= 0 || lease > domain.MaxLease || limit < 1 || limit > domain.MaxClaimBatch {
		return nil, typed(shared.ErrorInvalid, "outbox.invalid_claim")
	}
	if _, err := shared.NewBoundedString(worker, 255); err != nil {
		return nil, typed(shared.ErrorInvalid, "outbox.invalid_claim")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `UPDATE public.outbox_messages
 SET state = 'dead_letter', claim_token = NULL, claimed_by = NULL, claimed_at = NULL, lease_expires_at = NULL,
     last_error_code = 'outbox.attempts_exhausted', dead_lettered_at = $2,
     retain_until = $2 + INTERVAL '90 days'
 WHERE tenant_id = $1::uuid AND state = 'claimed' AND lease_expires_at <= $2 AND attempt_count >= 1000`, tenantID.String(), claimedAt.UTC())
	if err != nil {
		return nil, unavailable()
	}
	rows, err := tx.Query(ctx, `WITH candidates AS (
 SELECT outbox_message_id AS candidate_id
   FROM public.outbox_messages
  WHERE tenant_id = $1::uuid AND attempt_count < 1000
    AND ((state IN ('pending', 'retry') AND available_at <= $4) OR (state = 'claimed' AND lease_expires_at <= $4))
  ORDER BY sequence
  FOR UPDATE SKIP LOCKED
  LIMIT $6
)
UPDATE public.outbox_messages message
   SET state = 'claimed', attempt_count = attempt_count + 1, claim_token = $3::uuid,
       claimed_by = $2, claimed_at = $4, lease_expires_at = $4 + ($5 * INTERVAL '1 microsecond')
  FROM candidates
 WHERE message.tenant_id = $1::uuid AND message.outbox_message_id = candidates.candidate_id
RETURNING `+returningMessage, tenantID.String(), worker, claimToken.String(), claimedAt.UTC(), lease.Microseconds(), limit)
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.Message, 0, limit)
	for rows.Next() {
		value, scanErr := scan(rows)
		if scanErr != nil {
			return nil, unavailable()
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, unavailable()
	}
	sort.Slice(values, func(left, right int) bool { return values[left].Sequence() < values[right].Sequence() })
	if err := tx.Commit(ctx); err != nil {
		return nil, unavailable()
	}
	return values, nil
}

func (repository *Repository) Retry(ctx context.Context, tenantID, messageID, claimToken shared.UUID,
	reason shared.ReasonCode, availableAt time.Time,
) (domain.Message, error) {
	if reason.String() == "" || availableAt.IsZero() {
		return domain.Message{}, typed(shared.ErrorInvalid, "outbox.invalid_retry")
	}
	return repository.finish(ctx, tenantID, messageID, claimToken, `state = 'retry', claim_token = NULL, claimed_by = NULL,
 claimed_at = NULL, lease_expires_at = NULL, available_at = $5::timestamptz`, reason.String(), availableAt.UTC())
}

func (repository *Repository) Deliver(ctx context.Context, tenantID, messageID, claimToken shared.UUID, deliveredAt time.Time) (domain.Message, error) {
	if deliveredAt.IsZero() {
		return domain.Message{}, typed(shared.ErrorInvalid, "outbox.invalid_delivery")
	}
	return repository.finish(ctx, tenantID, messageID, claimToken, `state = 'delivered', claim_token = NULL, claimed_by = NULL,
 claimed_at = NULL, lease_expires_at = NULL, delivered_at = $5::timestamptz,
 retain_until = $5::timestamptz + INTERVAL '30 days'`, nil, deliveredAt.UTC())
}

func (repository *Repository) DeadLetter(ctx context.Context, tenantID, messageID, claimToken shared.UUID,
	reason shared.ReasonCode, deadLetteredAt time.Time,
) (domain.Message, error) {
	if reason.String() == "" || deadLetteredAt.IsZero() {
		return domain.Message{}, typed(shared.ErrorInvalid, "outbox.invalid_dead_letter")
	}
	return repository.finish(ctx, tenantID, messageID, claimToken, `state = 'dead_letter', claim_token = NULL, claimed_by = NULL,
 claimed_at = NULL, lease_expires_at = NULL, dead_lettered_at = $5::timestamptz,
 retain_until = $5::timestamptz + INTERVAL '90 days'`, reason.String(), deadLetteredAt.UTC())
}

func (repository *Repository) finish(ctx context.Context, tenantID, messageID, claimToken shared.UUID,
	assignment string, reason any, at time.Time,
) (domain.Message, error) {
	if !validUUID(messageID) || !validUUID(claimToken) {
		return domain.Message{}, typed(shared.ErrorInvalid, "outbox.invalid_completion")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Message{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, `UPDATE public.outbox_messages SET last_error_code = COALESCE($4::text, last_error_code), `+assignment+`
 WHERE tenant_id = $1::uuid AND outbox_message_id = $2::uuid AND state = 'claimed' AND claim_token = $3::uuid
 RETURNING `+returningMessage, tenantID.String(), messageID.String(), claimToken.String(), reason, at))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, typed(shared.ErrorConflict, "outbox.stale_claim")
	}
	if err != nil {
		return domain.Message{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Message{}, unavailable()
	}
	return value, nil
}

const returningMessage = `tenant_id::text, outbox_message_id::text, sequence, event_source, event_type, event_subject,
       event_payload, payload_digest, state, attempt_count, available_at, claim_token::text,
       claimed_by, claimed_at, lease_expires_at, last_error_code, delivered_at, dead_lettered_at,
       retain_until, created_at`
const selectMessage = `SELECT ` + returningMessage + ` FROM public.outbox_messages`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Message, error) {
	var tenantValue, idValue, source, eventType, subject, digestValue, stateValue string
	var sequence int64
	var payload []byte
	var attempts int32
	var availableAt, createdAt time.Time
	var tokenValue, worker, errorValue *string
	var claimedAt, leaseUntil, deliveredAt, deadAt, retainUntil *time.Time
	if err := row.Scan(&tenantValue, &idValue, &sequence, &source, &eventType, &subject, &payload, &digestValue,
		&stateValue, &attempts, &availableAt, &tokenValue, &worker, &claimedAt, &leaseUntil, &errorValue,
		&deliveredAt, &deadAt, &retainUntil, &createdAt); err != nil {
		return domain.Message{}, err
	}
	tenantID, err := shared.ParseUUID(tenantValue)
	if err != nil {
		return domain.Message{}, err
	}
	id, err := shared.ParseUUID(idValue)
	if err != nil {
		return domain.Message{}, err
	}
	digest, err := shared.ParseDigest(digestValue)
	if err != nil {
		return domain.Message{}, err
	}
	var token *shared.UUID
	if tokenValue != nil {
		parsed, parseErr := shared.ParseUUID(*tokenValue)
		if parseErr != nil {
			return domain.Message{}, parseErr
		}
		token = &parsed
	}
	var reason *shared.ReasonCode
	if errorValue != nil {
		parsed, parseErr := shared.NewReasonCode(*errorValue)
		if parseErr != nil {
			return domain.Message{}, parseErr
		}
		reason = &parsed
	}
	workerValue := ""
	if worker != nil {
		workerValue = *worker
	}
	if sequence < 1 || attempts < 0 || attempts > domain.MaxAttempts {
		return domain.Message{}, fmt.Errorf("invalid outbox database values")
	}
	return domain.Restore(tenantID, id, uint64(sequence), source, eventType, subject, payload, digest, domain.State(stateValue),
		uint16(attempts), availableAt, token, workerValue, claimedAt, leaseUntil, reason, deliveredAt, deadAt, retainUntil, createdAt)
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "outbox.invalid_tenant")
	}
	tx, err := repository.db.Begin(ctx)
	if err != nil {
		return nil, unavailable()
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('thinkpixelmp.tenant_id', $1, true)`, tenantID.String()); err != nil {
		rollback(tx)
		return nil, unavailable()
	}
	return tx, nil
}

func rollback(tx pgx.Tx)            { _ = tx.Rollback(context.Background()) }
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }
func typed(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}
func unavailable() error { return typed(shared.ErrorUnavailable, "outbox.persistence_unavailable") }

var _ domain.Repository = (*Repository)(nil)
