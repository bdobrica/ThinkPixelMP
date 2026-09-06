// Package artifactsource persists tenant-scoped ArtifactSource values in PostgreSQL.
package artifactsource

import (
	"context"
	"errors"
	"fmt"

	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
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
		return nil, fmt.Errorf("artifact source repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.ArtifactSource) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	var importRecordID any
	if identifier, ok := value.ImportRecordID(); ok {
		importRecordID = identifier.String()
	}
	result, err := tx.Exec(ctx, `INSERT INTO public.artifact_sources
  (tenant_id, artifact_version_id, kind, submitted_reference, resolved_reference, resolved_digest, endpoint, import_record_id, delivery_model)
 SELECT av.tenant_id, av.artifact_version_id, $3, NULLIF($4, ''), $5, av.resolved_digest, NULLIF($7, ''), $8::uuid, av.delivery_model
   FROM public.artifact_versions av
  WHERE av.tenant_id = $1::uuid AND av.artifact_version_id = $2::uuid AND av.resolved_digest = $6
    AND (($3 = 'oci' AND av.delivery_model = 'oci')
      OR ($3 = 'remote' AND av.delivery_model = 'remote')
      OR ($3 = 'import-record' AND av.delivery_model = 'imported-source'))`,
		value.TenantID().String(), value.ArtifactVersionID().String(), string(value.Kind()), value.SubmittedReference(),
		value.ResolvedReference(), value.ResolvedDigest().String(), value.Endpoint(), importRecordID)
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact_source.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact_source.artifact_version_not_found")
	}
	digest := value.ResolvedDigest()
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactSourceCreated,
		ResourceType: "artifact_source", ResourceID: value.ArtifactVersionID().String(), ArtifactDigest: &digest,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactVersionID shared.UUID) (domain.ArtifactSource, error) {
	if !validUUID(artifactVersionID) {
		return domain.ArtifactSource{}, typed(shared.ErrorInvalid, "artifact_source.invalid_artifact_version_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactSource{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, `SELECT tenant_id::text, artifact_version_id::text, kind,
       COALESCE(submitted_reference, ''), resolved_reference, resolved_digest, COALESCE(endpoint, ''), import_record_id::text
  FROM public.artifact_sources
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantID.String(), artifactVersionID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactSource{}, typed(shared.ErrorNotFound, "artifact_source.not_found")
	}
	if err != nil {
		return domain.ArtifactSource{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactSource{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "artifact_source.invalid_tenant")
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

func scan(row scanner) (domain.ArtifactSource, error) {
	var tenant, artifactVersion, kind, submitted, resolved, digest, endpoint string
	var importRecord *string
	if err := row.Scan(&tenant, &artifactVersion, &kind, &submitted, &resolved, &digest, &endpoint, &importRecord); err != nil {
		return domain.ArtifactSource{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.ArtifactSource{}, err
	}
	versionID, err := shared.ParseUUID(artifactVersion)
	if err != nil {
		return domain.ArtifactSource{}, err
	}
	resolvedDigest, err := shared.ParseDigest(digest)
	if err != nil {
		return domain.ArtifactSource{}, err
	}
	var importRecordID *shared.UUID
	if importRecord != nil {
		parsed, err := shared.ParseUUID(*importRecord)
		if err != nil {
			return domain.ArtifactSource{}, err
		}
		importRecordID = &parsed
	}
	return domain.Restore(tenantID, versionID, domain.Kind(kind), submitted, resolved, resolvedDigest, endpoint, importRecordID)
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
	return typed(shared.ErrorUnavailable, "artifact_source.persistence_unavailable")
}
func validUUID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

var _ domain.Repository = (*Repository)(nil)
