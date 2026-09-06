// Package artifactdependency persists tenant-scoped ArtifactDependency declarations in PostgreSQL.
package artifactdependency

import (
	"context"
	"errors"
	"fmt"

	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdependency"
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
		return nil, fmt.Errorf("artifact dependency repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.ArtifactDependency) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	catalog, hasCatalog := value.Catalog()
	if !hasCatalog {
		catalog = ""
	}
	source, hasSource := value.Source()
	if !hasSource {
		source = ""
	}
	result, err := tx.Exec(ctx, `INSERT INTO public.artifact_dependencies
  (tenant_id, artifact_version_id, dependency_index, schema_version, dependency_name,
   artifact_identity, required, catalog, source, selector_kind, selector_value,
   dependency_digest, normalized_bytes, normalized_dependency)
 SELECT d.tenant_id, d.artifact_version_id, $3::smallint, 1, $4, $5, $6,
        NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12::bytea,
        convert_from($12::bytea, 'UTF8')::jsonb
   FROM public.artifact_descriptors d
  WHERE d.tenant_id = $1::uuid AND d.artifact_version_id = $2::uuid
    AND d.normalized_metadata -> 'dependencies' -> $3::integer = convert_from($12::bytea, 'UTF8')::jsonb`,
		value.TenantID().String(), value.ArtifactVersionID().String(), value.Index(), value.Name(), value.Artifact().String(), value.Required(), catalog, source, string(value.SelectorKind()), value.SelectorValue(), value.DependencyDigest().String(), value.NormalizedDependency())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact_dependency.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact_dependency.descriptor_not_found")
	}
	versionID := value.ArtifactVersionID()
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactDependencyCreated,
		ResourceType: "artifact_dependency", ResourceID: fmt.Sprintf("%s:%d", versionID, value.Index()), ArtifactVersionID: &versionID,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactVersionID shared.UUID, index uint16) (domain.ArtifactDependency, error) {
	if !validUUID(artifactVersionID) || index >= domain.MaxDependencies {
		return domain.ArtifactDependency{}, typed(shared.ErrorInvalid, "artifact_dependency.invalid_key")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactDependency{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, selectDependency+` WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid AND dependency_index = $3`, tenantID.String(), artifactVersionID.String(), index))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactDependency{}, typed(shared.ErrorNotFound, "artifact_dependency.not_found")
	}
	if err != nil {
		return domain.ArtifactDependency{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactDependency{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID, artifactVersionID shared.UUID) ([]domain.ArtifactDependency, error) {
	if !validUUID(artifactVersionID) {
		return nil, typed(shared.ErrorInvalid, "artifact_dependency.invalid_artifact_version_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	rows, err := tx.Query(ctx, selectDependency+` WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid ORDER BY dependency_index`, tenantID.String(), artifactVersionID.String())
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.ArtifactDependency, 0)
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, unavailable()
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, unavailable()
	}
	return values, nil
}

const selectDependency = `SELECT tenant_id::text, artifact_version_id::text, dependency_index,
       dependency_digest, normalized_bytes
  FROM public.artifact_dependencies`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.ArtifactDependency, error) {
	var tenant, version, digest string
	var index uint16
	var metadata []byte
	if err := row.Scan(&tenant, &version, &index, &digest, &metadata); err != nil {
		return domain.ArtifactDependency{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.ArtifactDependency{}, err
	}
	versionID, err := shared.ParseUUID(version)
	if err != nil {
		return domain.ArtifactDependency{}, err
	}
	dependencyDigest, err := shared.ParseDigest(digest)
	if err != nil {
		return domain.ArtifactDependency{}, err
	}
	return domain.Restore(tenantID, versionID, index, dependencyDigest, metadata)
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "artifact_dependency.invalid_tenant")
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
	return typed(shared.ErrorUnavailable, "artifact_dependency.persistence_unavailable")
}
func validUUID(id shared.UUID) bool { _, err := id.MarshalText(); return err == nil }

var _ domain.Repository = (*Repository)(nil)
