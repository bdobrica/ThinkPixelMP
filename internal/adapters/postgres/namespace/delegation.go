package namespace

import (
	"context"
	"errors"
	"time"

	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *Repository) CreateDelegation(ctx context.Context, value domain.Delegation) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `INSERT INTO public.namespace_delegations
  (tenant_id, delegation_id, namespace_id, child_prefix, publisher_id, created_at)
 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6)`, value.TenantID().String(),
		value.ID().String(), value.NamespaceID().String(), value.ChildPrefix(), value.PublisherID().String(), value.CreatedAt())
	if err != nil {
		return delegationWriteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.namespace_delegation_state_records
  (tenant_id, delegation_id, child_prefix, version, state, recorded_at)
 VALUES ($1::uuid, $2::uuid, $3, 1, 'active', $4)`, value.TenantID().String(), value.ID().String(), value.ChildPrefix(), value.CreatedAt())
	if err != nil {
		return unavailable()
	}
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionNamespaceDelegated,
		ResourceType: "namespace_delegation", ResourceID: value.ID().String(),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) GetDelegation(ctx context.Context, tenantID, delegationID shared.UUID) (domain.Delegation, error) {
	if !validUUID(delegationID) {
		return domain.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Delegation{}, err
	}
	defer rollback(tx)
	value, err := scanDelegation(tx.QueryRow(ctx, delegationSelect+` WHERE d.tenant_id = $1::uuid AND d.delegation_id = $2::uuid`, tenantID.String(), delegationID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Delegation{}, typed(shared.ErrorNotFound, "namespace.delegation_not_found")
	}
	if err != nil {
		return domain.Delegation{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Delegation{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) RevokeDelegation(ctx context.Context, tenantID, delegationID shared.UUID, expectedVersion int64, reason shared.ReasonCode, explanation string, at time.Time) (domain.Delegation, error) {
	if !validUUID(delegationID) || expectedVersion < 1 || reason.String() == "" || at.IsZero() {
		return domain.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_revocation")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Delegation{}, err
	}
	defer rollback(tx)
	current, err := scanDelegation(tx.QueryRow(ctx, delegationSelect+` WHERE d.tenant_id = $1::uuid AND d.delegation_id = $2::uuid FOR UPDATE OF d`, tenantID.String(), delegationID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Delegation{}, typed(shared.ErrorNotFound, "namespace.delegation_not_found")
	}
	if err != nil {
		return domain.Delegation{}, unavailable()
	}
	if current.StateVersion() != expectedVersion {
		return domain.Delegation{}, typed(shared.ErrorConflict, "namespace.stale_delegation_version")
	}
	updated, err := current.Revoke(reason, explanation, at)
	if err != nil {
		return domain.Delegation{}, typed(shared.ErrorConflict, "namespace.invalid_delegation_transition")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.namespace_delegation_state_records
  (tenant_id, delegation_id, child_prefix, version, state, reason_code, explanation, recorded_at)
 VALUES ($1::uuid, $2::uuid, $3, $4, 'revoked', $5, $6, $7)`, tenantID.String(), delegationID.String(),
		current.ChildPrefix(), updated.StateVersion(), reason.String(), explanation, at.UTC()); err != nil {
		return domain.Delegation{}, unavailable()
	}
	result, err := tx.Exec(ctx, `UPDATE public.namespace_delegations
 SET current_state_version = $3, revoked_at = $4
 WHERE tenant_id = $1::uuid AND delegation_id = $2::uuid AND current_state_version = $5`, tenantID.String(), delegationID.String(), updated.StateVersion(), at.UTC(), expectedVersion)
	if err != nil {
		return domain.Delegation{}, unavailable()
	}
	if result.RowsAffected() != 1 {
		return domain.Delegation{}, typed(shared.ErrorConflict, "namespace.stale_delegation_version")
	}
	decision, _ := shared.NewReasonCode("revoked")
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: tenantID, Action: postgresaudit.ActionNamespaceDelegationRevoked,
		ResourceType: "namespace_delegation", ResourceID: delegationID.String(), Decision: &decision,
		ReasonCodes: []shared.ReasonCode{reason},
	}); err != nil {
		return domain.Delegation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Delegation{}, unavailable()
	}
	return updated, nil
}

// ResolveOwner returns the verified Publisher controlling the longest namespace
// ownership or active delegation prefix. It fails closed on missing authority.
func (repository *Repository) ResolveOwner(ctx context.Context, tenantID shared.UUID, path string) (shared.UUID, error) {
	if err := domain.ValidatePath(path); err != nil {
		return shared.UUID{}, typed(shared.ErrorInvalid, "namespace.invalid_path")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return shared.UUID{}, err
	}
	defer rollback(tx)
	var owner string
	err = tx.QueryRow(ctx, `WITH candidates AS (
 SELECT n.path AS prefix, n.owner_publisher_id AS publisher_id, s.state
 FROM public.namespaces n
 JOIN public.publishers p ON p.tenant_id = n.tenant_id AND p.publisher_id = n.owner_publisher_id
 JOIN public.publisher_state_records s ON s.tenant_id = p.tenant_id AND s.publisher_id = p.publisher_id AND s.version = p.current_state_version
 WHERE n.tenant_id = $1::uuid AND ($2 = n.path OR $2 LIKE n.path || '/%')
 UNION ALL
 SELECT d.child_prefix, d.publisher_id, s.state
 FROM public.namespace_delegations d
 JOIN public.publishers p ON p.tenant_id = d.tenant_id AND p.publisher_id = d.publisher_id
 JOIN public.publisher_state_records s ON s.tenant_id = p.tenant_id AND s.publisher_id = p.publisher_id AND s.version = p.current_state_version
 WHERE d.tenant_id = $1::uuid AND d.revoked_at IS NULL
   AND ($2 = d.child_prefix OR $2 LIKE d.child_prefix || '/%')
), controlling AS (
 SELECT publisher_id, state FROM candidates ORDER BY char_length(prefix) DESC LIMIT 1
)
SELECT publisher_id::text FROM controlling WHERE state = 'verified'`, tenantID.String(), path).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.UUID{}, typed(shared.ErrorForbidden, "namespace.publication_not_authorized")
	}
	if err != nil {
		return shared.UUID{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return shared.UUID{}, unavailable()
	}
	parsed, err := shared.ParseUUID(owner)
	if err != nil {
		return shared.UUID{}, unavailable()
	}
	return parsed, nil
}

const delegationSelect = `SELECT d.tenant_id::text, d.delegation_id::text, d.namespace_id::text,
       d.publisher_id::text, d.child_prefix, s.state, d.current_state_version, d.created_at
  FROM public.namespace_delegations d
  JOIN public.namespace_delegation_state_records s
    ON s.tenant_id = d.tenant_id AND s.delegation_id = d.delegation_id AND s.version = d.current_state_version`

func scanDelegation(row scanner) (domain.Delegation, error) {
	var tenant, identifier, namespaceID, publisherID, prefix, state string
	var version int64
	var createdAt time.Time
	if err := row.Scan(&tenant, &identifier, &namespaceID, &publisherID, &prefix, &state, &version, &createdAt); err != nil {
		return domain.Delegation{}, err
	}
	values := make([]shared.UUID, 4)
	for index, raw := range []string{tenant, identifier, namespaceID, publisherID} {
		parsed, err := shared.ParseUUID(raw)
		if err != nil {
			return domain.Delegation{}, err
		}
		values[index] = parsed
	}
	return domain.RestoreDelegation(values[0], values[1], values[2], values[3], prefix, domain.DelegationState(state), version, createdAt)
}

func delegationWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		if postgresError.Code == "23505" {
			return typed(shared.ErrorConflict, "namespace.delegation_conflict")
		}
		if postgresError.Code == "23503" || postgresError.Code == "23514" {
			return typed(shared.ErrorConflict, "namespace.invalid_delegation")
		}
	}
	return unavailable()
}

var _ domain.DelegationRepository = (*Repository)(nil)
