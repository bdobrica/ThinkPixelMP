// Package artifactdescriptor persists tenant-scoped ArtifactDescriptor values in PostgreSQL.
package artifactdescriptor

import (
	"context"
	"errors"
	"fmt"

	postgres "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres"
	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdescriptor"
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
		return nil, fmt.Errorf("artifact descriptor repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.ArtifactDescriptor) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `INSERT INTO public.artifact_descriptors
  (tenant_id, artifact_version_id, artifact_id, namespace_id, schema_version, kind, namespace_path,
   artifact_name, semantic_version, media_type, descriptor_digest, normalized_bytes, normalized_metadata)
 SELECT av.tenant_id, av.artifact_version_id, av.artifact_id, a.namespace_id, 1, av.kind, n.path,
        a.name, av.semantic_version, $3, $4, $5::bytea, convert_from($5::bytea, 'UTF8')::jsonb
   FROM public.artifact_versions av
   JOIN public.artifacts a ON a.tenant_id = av.tenant_id AND a.artifact_id = av.artifact_id AND a.kind = av.kind
   JOIN public.namespaces n ON n.tenant_id = a.tenant_id AND n.namespace_id = a.namespace_id
  WHERE av.tenant_id = $1::uuid AND av.artifact_version_id = $2::uuid
    AND av.kind = $6 AND n.path = $7 AND a.name = $8 AND av.semantic_version = $9`,
		value.TenantID().String(), value.ArtifactVersionID().String(), value.MediaType(), value.DescriptorDigest().String(),
		value.NormalizedMetadata(), string(value.Kind()), value.Identity().Namespace(), value.Identity().Name(), value.SemanticVersion().String())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact_descriptor.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact_descriptor.artifact_version_not_found")
	}
	versionID := value.ArtifactVersionID()
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactDescriptorCreated,
		ResourceType: "artifact_descriptor", ResourceID: versionID.String(), ArtifactVersionID: &versionID,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactVersionID shared.UUID) (domain.ArtifactDescriptor, error) {
	if !validUUID(artifactVersionID) {
		return domain.ArtifactDescriptor{}, typed(shared.ErrorInvalid, "artifact_descriptor.invalid_artifact_version_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactDescriptor{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, `SELECT tenant_id::text, artifact_version_id::text, descriptor_digest, media_type, normalized_bytes
  FROM public.artifact_descriptors
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantID.String(), artifactVersionID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactDescriptor{}, typed(shared.ErrorNotFound, "artifact_descriptor.not_found")
	}
	if err != nil {
		return domain.ArtifactDescriptor{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactDescriptor{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "artifact_descriptor.invalid_tenant")
	}
	return postgres.BeginRepositoryTransaction(ctx, repository.db, tenantID)
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.ArtifactDescriptor, error) {
	var tenant, version, digest, mediaType string
	var metadata []byte
	if err := row.Scan(&tenant, &version, &digest, &mediaType, &metadata); err != nil {
		return domain.ArtifactDescriptor{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.ArtifactDescriptor{}, err
	}
	versionID, err := shared.ParseUUID(version)
	if err != nil {
		return domain.ArtifactDescriptor{}, err
	}
	descriptorDigest, err := shared.ParseDigest(digest)
	if err != nil {
		return domain.ArtifactDescriptor{}, err
	}
	return domain.Restore(tenantID, versionID, descriptorDigest, mediaType, metadata)
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
	return typed(shared.ErrorUnavailable, "artifact_descriptor.persistence_unavailable")
}
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }

var _ domain.Repository = (*Repository)(nil)
