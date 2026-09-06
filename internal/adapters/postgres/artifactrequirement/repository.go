// Package artifactrequirement persists tenant-scoped ArtifactRequirement values in PostgreSQL.
package artifactrequirement

import (
	"context"
	"errors"
	"fmt"

	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactrequirement"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct{ db beginner }

func NewRepository(db beginner) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("artifact requirement repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.ArtifactRequirement) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `INSERT INTO public.artifact_requirements
  (tenant_id, artifact_version_id, schema_version, requirement_digest, normalized_bytes, normalized_requirement)
 SELECT d.tenant_id, d.artifact_version_id, 1, $3, $4::bytea, convert_from($4::bytea, 'UTF8')::jsonb
   FROM public.artifact_descriptors d
  WHERE d.tenant_id = $1::uuid AND d.artifact_version_id = $2::uuid
    AND d.normalized_metadata -> 'requirements' = convert_from($4::bytea, 'UTF8')::jsonb`,
		value.TenantID().String(), value.ArtifactVersionID().String(), value.RequirementDigest().String(), value.NormalizedRequirement())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact_requirement.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact_requirement.descriptor_not_found")
	}
	versionID := value.ArtifactVersionID()
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactRequirementCreated,
		ResourceType: "artifact_requirement", ResourceID: versionID.String(), ArtifactVersionID: &versionID,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactVersionID shared.UUID) (domain.ArtifactRequirement, error) {
	if !validUUID(artifactVersionID) {
		return domain.ArtifactRequirement{}, typed(shared.ErrorInvalid, "artifact_requirement.invalid_artifact_version_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactRequirement{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, `SELECT tenant_id::text, artifact_version_id::text, requirement_digest, normalized_bytes
  FROM public.artifact_requirements
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantID.String(), artifactVersionID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactRequirement{}, typed(shared.ErrorNotFound, "artifact_requirement.not_found")
	}
	if err != nil {
		return domain.ArtifactRequirement{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactRequirement{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "artifact_requirement.invalid_tenant")
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

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.ArtifactRequirement, error) {
	var tenant, version, digest string
	var metadata []byte
	if err := row.Scan(&tenant, &version, &digest, &metadata); err != nil {
		return domain.ArtifactRequirement{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.ArtifactRequirement{}, err
	}
	versionID, err := shared.ParseUUID(version)
	if err != nil {
		return domain.ArtifactRequirement{}, err
	}
	requirementDigest, err := shared.ParseDigest(digest)
	if err != nil {
		return domain.ArtifactRequirement{}, err
	}
	return domain.Restore(tenantID, versionID, requirementDigest, metadata)
}

func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }
func uniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
func typed(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}
func unavailable() error {
	return typed(shared.ErrorUnavailable, "artifact_requirement.persistence_unavailable")
}
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }

var _ domain.Repository = (*Repository)(nil)
