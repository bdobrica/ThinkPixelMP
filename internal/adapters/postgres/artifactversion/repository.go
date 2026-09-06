// Package artifactversion persists tenant-scoped ArtifactVersion aggregates in PostgreSQL.
package artifactversion

import (
	"context"
	"errors"
	"fmt"
	"time"

	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
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
		return nil, fmt.Errorf("artifact version repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.ArtifactVersion) error {
	if value.Lifecycle() != domain.LifecycleActive {
		return typed(shared.ErrorInvalid, "artifact_version.invalid_lifecycle")
	}
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `INSERT INTO public.artifact_versions
  (tenant_id, artifact_version_id, artifact_id, publisher_id, semantic_version, resolved_digest, kind, artifact_class, delivery_model, lifecycle, registered_at)
 SELECT $1::uuid, $2::uuid, a.artifact_id, p.publisher_id, $5, $6, a.kind, $8, $9, $10, $11
   FROM public.artifacts a
   JOIN public.publishers p ON p.tenant_id = a.tenant_id AND p.publisher_id = $4::uuid
  WHERE a.tenant_id = $1::uuid AND a.artifact_id = $3::uuid AND a.kind = $7`,
		value.TenantID().String(), value.ID().String(), value.ArtifactID().String(), value.PublisherID().String(),
		value.SemanticVersion().String(), value.Digest().String(), string(value.Kind()), string(value.Class()),
		string(value.DeliveryModel()), string(value.Lifecycle()), value.RegisteredAt())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact_version.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact_version.artifact_or_publisher_not_found")
	}
	digest := value.Digest()
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactVersionRegistered,
		ResourceType: "artifact_version", ResourceID: value.ID().String(), ArtifactDigest: &digest,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactVersionID shared.UUID) (domain.ArtifactVersion, error) {
	if !validUUID(artifactVersionID) {
		return domain.ArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_id")
	}
	return repository.get(ctx, tenantID, `artifact_version_id = $2::uuid`, artifactVersionID.String())
}

func (repository *Repository) GetByDigest(ctx context.Context, tenantID shared.UUID, digest shared.Digest) (domain.ArtifactVersion, error) {
	if _, err := digest.MarshalText(); err != nil {
		return domain.ArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_digest")
	}
	return repository.get(ctx, tenantID, `resolved_digest = $2`, digest.String())
}

func (repository *Repository) GetBySemanticVersion(ctx context.Context, tenantID, artifactID shared.UUID, semanticVersion domain.SemanticVersion) (domain.ArtifactVersion, error) {
	if !validUUID(artifactID) {
		return domain.ArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_artifact_id")
	}
	if _, err := domain.ParseSemanticVersion(semanticVersion.String()); err != nil {
		return domain.ArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_semantic_version")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, artifactVersionSelect+` WHERE tenant_id = $1::uuid AND artifact_id = $2::uuid AND semantic_version = $3`, tenantID.String(), artifactID.String(), semanticVersion.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactVersion{}, typed(shared.ErrorNotFound, "artifact_version.not_found")
	}
	if err != nil {
		return domain.ArtifactVersion{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactVersion{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) get(ctx context.Context, tenantID shared.UUID, predicate, argument string) (domain.ArtifactVersion, error) {
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, artifactVersionSelect+` WHERE tenant_id = $1::uuid AND `+predicate, tenantID.String(), argument))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArtifactVersion{}, typed(shared.ErrorNotFound, "artifact_version.not_found")
	}
	if err != nil {
		return domain.ArtifactVersion{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ArtifactVersion{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID, artifactID shared.UUID, after *shared.UUID, limit int) ([]domain.ArtifactVersion, error) {
	if !validUUID(artifactID) {
		return nil, typed(shared.ErrorInvalid, "artifact_version.invalid_artifact_id")
	}
	if limit < 1 || limit > domain.MaxListSize {
		return nil, typed(shared.ErrorInvalid, "artifact_version.invalid_page_size")
	}
	if after != nil && !validUUID(*after) {
		return nil, typed(shared.ErrorInvalid, "artifact_version.invalid_cursor")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	query := artifactVersionSelect + ` WHERE tenant_id = $1::uuid AND artifact_id = $2::uuid`
	arguments := []any{tenantID.String(), artifactID.String()}
	if after != nil {
		query += ` AND artifact_version_id > $3::uuid`
		arguments = append(arguments, after.String())
	}
	arguments = append(arguments, limit)
	query += ` ORDER BY artifact_version_id LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.ArtifactVersion, 0, limit)
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

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if !validUUID(tenantID) {
		return nil, typed(shared.ErrorInvalid, "artifact_version.invalid_tenant")
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

const artifactVersionSelect = `SELECT tenant_id::text, artifact_version_id::text, artifact_id::text, publisher_id::text,
       semantic_version, resolved_digest, kind, artifact_class, delivery_model, lifecycle, registered_at
  FROM public.artifact_versions`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.ArtifactVersion, error) {
	var tenant, identifier, artifactIdentifier, publisherIdentifier, semantic, digest, kind, class, delivery, lifecycle string
	var registeredAt time.Time
	if err := row.Scan(&tenant, &identifier, &artifactIdentifier, &publisherIdentifier, &semantic, &digest, &kind, &class, &delivery, &lifecycle, &registeredAt); err != nil {
		return domain.ArtifactVersion{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	versionID, err := shared.ParseUUID(identifier)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	artifactID, err := shared.ParseUUID(artifactIdentifier)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	publisherID, err := shared.ParseUUID(publisherIdentifier)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	semanticVersion, err := domain.ParseSemanticVersion(semantic)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	contentDigest, err := shared.ParseDigest(digest)
	if err != nil {
		return domain.ArtifactVersion{}, err
	}
	return domain.Restore(tenantID, versionID, artifactID, publisherID, semanticVersion, contentDigest, artifact.Kind(kind), domain.Class(class), domain.DeliveryModel(delivery), domain.Lifecycle(lifecycle), registeredAt)
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
	return typed(shared.ErrorUnavailable, "artifact_version.persistence_unavailable")
}
func validUUID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

var _ domain.Repository = (*Repository)(nil)
