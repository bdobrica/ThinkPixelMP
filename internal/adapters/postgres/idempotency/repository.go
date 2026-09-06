// Package idempotency persists tenant-scoped mutation idempotency records.
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	postgres "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct{ db beginner }

func NewRepository(db beginner) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("idempotency repository: database is required")
	}
	return &Repository{db: db}, nil
}

// Acquire claims an ownership tuple or returns its established record. A false
// created result means an identical request already owns the key.
func (repository *Repository) Acquire(ctx context.Context, value domain.Record) (domain.Record, bool, error) {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return domain.Record{}, false, err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `INSERT INTO public.idempotency_records
  (tenant_id, idempotency_record_id, principal_id, action, idempotency_key,
   request_digest, created_at, expires_at)
 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
 ON CONFLICT (tenant_id, principal_id, action, idempotency_key) DO NOTHING`,
		value.TenantID().String(), value.ID().String(), value.Principal(), value.Action().String(), value.Key(),
		value.RequestDigest().String(), value.CreatedAt(), value.ExpiresAt())
	if err != nil {
		return domain.Record{}, false, unavailable()
	}
	created := result.RowsAffected() == 1
	stored, err := scan(tx.QueryRow(ctx, selectRecord+ownershipWhere,
		value.TenantID().String(), value.Principal(), value.Action().String(), value.Key()))
	if err != nil {
		return domain.Record{}, false, unavailable()
	}
	if stored.RequestDigest() != value.RequestDigest() {
		return domain.Record{}, false, typed(shared.ErrorConflict, "idempotency.request_mismatch")
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Record{}, false, unavailable()
	}
	return stored, created, nil
}

func (repository *Repository) Get(ctx context.Context, tenantID shared.UUID, principal string, action shared.ReasonCode, key string) (domain.Record, error) {
	if !validOwnership(principal, action, key) {
		return domain.Record{}, typed(shared.ErrorInvalid, "idempotency.invalid_ownership")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Record{}, err
	}
	defer rollback(tx)
	value, err := get(ctx, tx, tenantID, principal, action, key)
	if err != nil {
		return domain.Record{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Record{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) Complete(ctx context.Context, tenantID shared.UUID, principal string, action shared.ReasonCode,
	key string, requestDigest shared.Digest, result domain.Result, completedAt time.Time,
) (domain.Record, error) {
	if !validOwnership(principal, action, key) || requestDigest.String() == "" || completedAt.IsZero() || result.Status() < 100 {
		return domain.Record{}, typed(shared.ErrorInvalid, "idempotency.invalid_completion")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Record{}, err
	}
	defer rollback(tx)
	resourceType, hasResource := result.ResourceType()
	resourceID, _ := result.ResourceID()
	var resourceTypeValue, resourceIDValue any
	if hasResource {
		resourceTypeValue, resourceIDValue = resourceType.String(), resourceID
	}
	row := tx.QueryRow(ctx, `UPDATE public.idempotency_records
   SET state = 'completed', response_status = $6, resource_type = $7, resource_id = $8, completed_at = $9
 WHERE tenant_id = $1::uuid AND principal_id = $2 AND action = $3 AND idempotency_key = $4
   AND request_digest = $5 AND state = 'pending'
 RETURNING `+returningRecord,
		tenantID.String(), principal, action.String(), key, requestDigest.String(), result.Status(), resourceTypeValue, resourceIDValue, completedAt.UTC())
	value, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		value, err = get(ctx, tx, tenantID, principal, action, key)
		if err != nil {
			return domain.Record{}, err
		}
		if value.RequestDigest() != requestDigest {
			return domain.Record{}, typed(shared.ErrorConflict, "idempotency.request_mismatch")
		}
		established, ok := value.Result()
		if !ok || !sameResult(established, result) {
			return domain.Record{}, typed(shared.ErrorConflict, "idempotency.result_mismatch")
		}
	} else if err != nil {
		return domain.Record{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Record{}, unavailable()
	}
	return value, nil
}

func get(ctx context.Context, tx pgx.Tx, tenantID shared.UUID, principal string, action shared.ReasonCode, key string) (domain.Record, error) {
	value, err := scan(tx.QueryRow(ctx, selectRecord+ownershipWhere, tenantID.String(), principal, action.String(), key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Record{}, typed(shared.ErrorNotFound, "idempotency.not_found")
	}
	if err != nil {
		return domain.Record{}, unavailable()
	}
	return value, nil
}

const returningRecord = `tenant_id::text, idempotency_record_id::text, principal_id, action, idempotency_key,
       request_digest, state, response_status, resource_type, resource_id,
       created_at, completed_at, expires_at`
const selectRecord = `SELECT ` + returningRecord + ` FROM public.idempotency_records`
const ownershipWhere = ` WHERE tenant_id = $1::uuid AND principal_id = $2 AND action = $3 AND idempotency_key = $4`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Record, error) {
	var tenantValue, idValue, principal, actionValue, key, digestValue, stateValue string
	var status *uint16
	var resourceTypeValue, resourceID *string
	var createdAt, expiresAt time.Time
	var completedAt *time.Time
	if err := row.Scan(&tenantValue, &idValue, &principal, &actionValue, &key, &digestValue, &stateValue,
		&status, &resourceTypeValue, &resourceID, &createdAt, &completedAt, &expiresAt); err != nil {
		return domain.Record{}, err
	}
	tenantID, err := shared.ParseUUID(tenantValue)
	if err != nil {
		return domain.Record{}, err
	}
	id, err := shared.ParseUUID(idValue)
	if err != nil {
		return domain.Record{}, err
	}
	action, err := shared.NewReasonCode(actionValue)
	if err != nil {
		return domain.Record{}, err
	}
	digest, err := shared.ParseDigest(digestValue)
	if err != nil {
		return domain.Record{}, err
	}
	var result *domain.Result
	if status != nil {
		resourceType := ""
		if resourceTypeValue != nil {
			resourceType = *resourceTypeValue
		}
		resource := ""
		if resourceID != nil {
			resource = *resourceID
		}
		parsed, err := domain.NewResult(*status, resourceType, resource)
		if err != nil {
			return domain.Record{}, err
		}
		result = &parsed
	}
	return domain.Restore(tenantID, id, principal, action, key, digest, domain.State(stateValue), result, createdAt, completedAt, expiresAt)
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "idempotency.invalid_tenant")
	}
	return postgres.BeginRepositoryTransaction(ctx, repository.db, tenantID)
}

func sameResult(left, right domain.Result) bool {
	leftType, leftHasType := left.ResourceType()
	rightType, rightHasType := right.ResourceType()
	leftID, leftHasID := left.ResourceID()
	rightID, rightHasID := right.ResourceID()
	return left.Status() == right.Status() && leftHasType == rightHasType && leftType == rightType &&
		leftHasID == rightHasID && leftID == rightID
}

func validOwnership(principal string, action shared.ReasonCode, key string) bool {
	return domain.ValidateOwnership(principal, action, key) == nil
}

func rollback(tx pgx.Tx)            { _ = tx.Rollback(context.Background()) }
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }
func typed(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}
func unavailable() error {
	return typed(shared.ErrorUnavailable, "idempotency.persistence_unavailable")
}

var _ domain.Repository = (*Repository)(nil)
