// Package audit persists tenant-scoped, append-only AuditEvent facts.
package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
)

const (
	ActionPublisherCreated           = "publisher.created"
	ActionPublisherStateChanged      = "publisher.state_changed"
	ActionNamespaceCreated           = "namespace.created"
	ActionArtifactCreated            = "artifact.created"
	ActionArtifactVersionRegistered  = "artifact_version.registered"
	ActionArtifactSourceCreated      = "artifact_source.created"
	ActionArtifactDescriptorCreated  = "artifact_descriptor.created"
	ActionArtifactRequirementCreated = "artifact_requirement.created"
	ActionArtifactDependencyCreated  = "artifact_dependency.created"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct{ db beginner }

type RecordParams struct {
	TenantID          shared.UUID
	Action            string
	ResourceType      string
	ResourceID        string
	ArtifactDigest    *shared.Digest
	ArtifactVersionID *shared.UUID
	Decision          *shared.ReasonCode
	ReasonCodes       []shared.ReasonCode
	EvidenceIDs       []shared.UUID
	PolicyDigests     []shared.Digest
}

func NewRepository(db beginner) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("audit repository: database is required")
	}
	return &Repository{db: db}, nil
}

// Record writes the audit fact through the caller's transaction. It never
// commits independently, which makes audit failure roll back the mutation.
func Record(ctx context.Context, tx pgx.Tx, params RecordParams) error {
	actor, ok := domain.ActorFromContext(ctx)
	if !ok {
		return typed(shared.ErrorUnauthorized, "audit.actor_required")
	}
	requestID, hasRequestID := actor.RequestID()
	var request any
	if hasRequestID {
		request = requestID.String()
	}
	var digest any
	if params.ArtifactDigest != nil {
		digest = params.ArtifactDigest.String()
	}
	var version any
	if params.ArtifactVersionID != nil {
		version = params.ArtifactVersionID.String()
	}
	var decision any
	if params.Decision != nil {
		decision = params.Decision.String()
	}
	reasons := make([]string, len(params.ReasonCodes))
	for index, reason := range params.ReasonCodes {
		reasons[index] = reason.String()
	}
	evidence := make([]string, len(params.EvidenceIDs))
	for index, identifier := range params.EvidenceIDs {
		evidence[index] = identifier.String()
	}
	policies := make([]string, len(params.PolicyDigests))
	for index, policy := range params.PolicyDigests {
		policies[index] = policy.String()
	}
	statement := `INSERT INTO public.audit_events
  (tenant_id, actor_id, action, resource_type, resource_id, artifact_digest, decision,
   reason_codes, evidence_ids, policy_digests, request_id, trace_id)
	SELECT $1::uuid, $2, $3, $4, $5, $6, $8, $9::text[], $10::uuid[], $11::text[], $12::uuid, NULLIF($13, '')
	  FROM (VALUES ($7::uuid)) AS ignored(version_id)`
	if params.ArtifactVersionID != nil {
		statement = `INSERT INTO public.audit_events
  (tenant_id, actor_id, action, resource_type, resource_id, artifact_digest, decision,
   reason_codes, evidence_ids, policy_digests, request_id, trace_id)
 SELECT $1::uuid, $2, $3, $4, $5, COALESCE($6, av.resolved_digest), $8,
        $9::text[], $10::uuid[], $11::text[], $12::uuid, NULLIF($13, '')
   FROM public.artifact_versions av
  WHERE av.tenant_id = $1::uuid AND av.artifact_version_id = $7::uuid`
	}
	result, err := tx.Exec(ctx, statement, params.TenantID.String(), actor.PrincipalID(), params.Action,
		params.ResourceType, params.ResourceID, digest, version, decision, reasons, evidence, policies, request, actor.TraceID())
	if err != nil || result.RowsAffected() != 1 {
		return typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, eventID shared.UUID) (domain.Event, error) {
	if !validUUID(eventID) {
		return domain.Event{}, typed(shared.ErrorInvalid, "audit.invalid_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Event{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, auditSelect+` WHERE tenant_id = $1::uuid AND audit_event_id = $2::uuid`, tenantID.String(), eventID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, typed(shared.ErrorNotFound, "audit.not_found")
	}
	if err != nil {
		return domain.Event{}, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]domain.Event, error) {
	if limit < 1 || limit > domain.MaxListSize || after != nil && !validUUID(*after) {
		return nil, typed(shared.ErrorInvalid, "audit.invalid_page")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	query := auditSelect + ` WHERE tenant_id = $1::uuid`
	arguments := []any{tenantID.String()}
	if after != nil {
		query += ` AND audit_event_id > $2::uuid`
		arguments = append(arguments, after.String())
	}
	arguments = append(arguments, limit)
	query += ` ORDER BY audit_event_id LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	defer rows.Close()
	values := make([]domain.Event, 0, limit)
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	return values, nil
}

const auditSelect = `SELECT tenant_id::text, audit_event_id::text, actor_id, action, resource_type, resource_id,
       artifact_digest, decision, reason_codes, evidence_ids::text[], policy_digests,
       occurred_at, request_id::text, COALESCE(trace_id, '')
  FROM public.audit_events`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Event, error) {
	var tenant, identifier, actor, actionValue, resourceTypeValue, resourceID, trace string
	var digestValue, decisionValue, requestValue *string
	var reasonValues, evidenceValues, policyValues []string
	var occurredAt time.Time
	if err := row.Scan(&tenant, &identifier, &actor, &actionValue, &resourceTypeValue, &resourceID, &digestValue,
		&decisionValue, &reasonValues, &evidenceValues, &policyValues, &occurredAt, &requestValue, &trace); err != nil {
		return domain.Event{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.Event{}, err
	}
	eventID, err := shared.ParseUUID(identifier)
	if err != nil {
		return domain.Event{}, err
	}
	action, err := shared.NewReasonCode(actionValue)
	if err != nil {
		return domain.Event{}, err
	}
	resourceType, err := shared.NewReasonCode(resourceTypeValue)
	if err != nil {
		return domain.Event{}, err
	}
	var digest *shared.Digest
	if digestValue != nil {
		parsed, err := shared.ParseDigest(*digestValue)
		if err != nil {
			return domain.Event{}, err
		}
		digest = &parsed
	}
	var decision *shared.ReasonCode
	if decisionValue != nil {
		parsed, err := shared.NewReasonCode(*decisionValue)
		if err != nil {
			return domain.Event{}, err
		}
		decision = &parsed
	}
	reasons := make([]shared.ReasonCode, len(reasonValues))
	for index, value := range reasonValues {
		reasons[index], err = shared.NewReasonCode(value)
		if err != nil {
			return domain.Event{}, err
		}
	}
	evidence := make([]shared.UUID, len(evidenceValues))
	for index, value := range evidenceValues {
		evidence[index], err = shared.ParseUUID(value)
		if err != nil {
			return domain.Event{}, err
		}
	}
	policies := make([]shared.Digest, len(policyValues))
	for index, value := range policyValues {
		policies[index], err = shared.ParseDigest(value)
		if err != nil {
			return domain.Event{}, err
		}
	}
	var request *shared.UUID
	if requestValue != nil {
		parsed, err := shared.ParseUUID(*requestValue)
		if err != nil {
			return domain.Event{}, err
		}
		request = &parsed
	}
	return domain.Restore(tenantID, eventID, actor, action, resourceType, resourceID, digest, decision, reasons, evidence, policies, occurredAt, request, trace)
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "audit.invalid_tenant")
	}
	tx, err := repository.db.Begin(ctx)
	if err != nil {
		return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('thinkpixelmp.tenant_id', $1, true)`, tenantID.String()); err != nil {
		rollback(tx)
		return nil, typed(shared.ErrorUnavailable, "audit.persistence_unavailable")
	}
	return tx, nil
}

func rollback(tx pgx.Tx)            { _ = tx.Rollback(context.Background()) }
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }
func typed(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}

var _ domain.Repository = (*Repository)(nil)
