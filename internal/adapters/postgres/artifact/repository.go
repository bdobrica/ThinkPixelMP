// Package artifact persists tenant-scoped logical Artifact aggregates in PostgreSQL.
package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	postgres "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres"
	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
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
		return nil, fmt.Errorf("artifact repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.Artifact) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	labels, err := json.Marshal(value.Labels())
	if err != nil {
		return typed(shared.ErrorInvalid, "artifact.invalid_labels")
	}
	result, err := tx.Exec(ctx, `INSERT INTO public.artifacts
  (tenant_id, artifact_id, namespace_id, name, kind, display_name, description, homepage, repository, labels, created_at)
 SELECT $1::uuid, $2::uuid, n.namespace_id, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10::jsonb, $11
   FROM public.namespaces n
  WHERE n.tenant_id = $1::uuid AND n.namespace_id = $3::uuid AND n.path = $12`,
		value.TenantID().String(), value.ID().String(), value.NamespaceID().String(), value.Name(), string(value.Kind()),
		value.DisplayName(), value.Description(), value.Homepage(), value.Repository(), string(labels), value.CreatedAt(), value.Identity().Namespace())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "artifact.conflict")
		}
		return unavailable()
	}
	if result.RowsAffected() != 1 {
		return typed(shared.ErrorNotFound, "artifact.namespace_not_found")
	}
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionArtifactCreated,
		ResourceType: "artifact", ResourceID: value.ID().String(),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, artifactID shared.UUID) (domain.Artifact, error) {
	if !validUUID(artifactID) {
		return domain.Artifact{}, typed(shared.ErrorInvalid, "artifact.invalid_id")
	}
	return repository.get(ctx, tenantID, `a.artifact_id = $2::uuid`, artifactID.String())
}

func (repository *Repository) GetByIdentity(ctx context.Context, tenantID shared.UUID, identity shared.ArtifactReference) (domain.Artifact, error) {
	if identity.String() == "" {
		return domain.Artifact{}, typed(shared.ErrorInvalid, "artifact.invalid_identity")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Artifact{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, artifactSelect+` WHERE a.tenant_id = $1::uuid AND n.path = $2 AND a.name = $3`, tenantID.String(), identity.Namespace(), identity.Name()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Artifact{}, typed(shared.ErrorNotFound, "artifact.not_found")
	}
	if err != nil {
		return domain.Artifact{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Artifact{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) get(ctx context.Context, tenantID shared.UUID, predicate, argument string) (domain.Artifact, error) {
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Artifact{}, err
	}
	defer rollback(tx)
	value, err := scan(tx.QueryRow(ctx, artifactSelect+` WHERE a.tenant_id = $1::uuid AND `+predicate, tenantID.String(), argument))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Artifact{}, typed(shared.ErrorNotFound, "artifact.not_found")
	}
	if err != nil {
		return domain.Artifact{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Artifact{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]domain.Artifact, error) {
	if limit < 1 || limit > domain.MaxListSize {
		return nil, typed(shared.ErrorInvalid, "artifact.invalid_page_size")
	}
	if after != nil && !validUUID(*after) {
		return nil, typed(shared.ErrorInvalid, "artifact.invalid_cursor")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	query := artifactSelect + ` WHERE a.tenant_id = $1::uuid`
	arguments := []any{tenantID.String()}
	if after != nil {
		query += ` AND a.artifact_id > $2::uuid`
		arguments = append(arguments, after.String())
	}
	arguments = append(arguments, limit)
	query += ` ORDER BY a.artifact_id LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.Artifact, 0, limit)
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
		return nil, typed(shared.ErrorInvalid, "artifact.invalid_tenant")
	}
	return postgres.BeginRepositoryTransaction(ctx, repository.db, tenantID)
}

const artifactSelect = `SELECT a.tenant_id::text, a.artifact_id::text, a.namespace_id::text, n.path, a.name, a.kind,
       COALESCE(a.display_name, ''), COALESCE(a.description, ''), COALESCE(a.homepage, ''), COALESCE(a.repository, ''), a.labels, a.created_at
  FROM public.artifacts a
  JOIN public.namespaces n ON n.tenant_id = a.tenant_id AND n.namespace_id = a.namespace_id`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Artifact, error) {
	var tenant, identifier, namespaceIdentifier, namespacePath, name, kind, displayName, description, homepage, sourceRepository string
	var labelsJSON []byte
	var createdAt time.Time
	if err := row.Scan(&tenant, &identifier, &namespaceIdentifier, &namespacePath, &name, &kind, &displayName, &description, &homepage, &sourceRepository, &labelsJSON, &createdAt); err != nil {
		return domain.Artifact{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.Artifact{}, err
	}
	artifactID, err := shared.ParseUUID(identifier)
	if err != nil {
		return domain.Artifact{}, err
	}
	namespaceID, err := shared.ParseUUID(namespaceIdentifier)
	if err != nil {
		return domain.Artifact{}, err
	}
	labels := map[string]string{}
	if err := json.Unmarshal(labelsJSON, &labels); err != nil {
		return domain.Artifact{}, err
	}
	return domain.Restore(tenantID, artifactID, namespaceID, namespacePath, name, domain.Kind(kind), displayName, description, homepage, sourceRepository, labels, createdAt)
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
func unavailable() error { return typed(shared.ErrorUnavailable, "artifact.persistence_unavailable") }
func validUUID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

var _ domain.Repository = (*Repository)(nil)
