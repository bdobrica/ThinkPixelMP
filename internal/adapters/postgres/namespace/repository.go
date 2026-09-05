// Package namespace persists tenant-scoped Namespace aggregates in PostgreSQL.
package namespace

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Repository scopes every operation with a transaction-local tenant setting and
// an explicit tenant predicate. db may be a pgx connection or pool.
type Repository struct{ db beginner }

func NewRepository(db beginner) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("namespace repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.Namespace) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `INSERT INTO public.namespaces
  (tenant_id, namespace_id, path, owner_publisher_id, created_at)
 VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5)`, value.TenantID().String(), value.ID().String(),
		value.Path(), value.OwnerPublisherID().String(), value.CreatedAt())
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "namespace.conflict")
		}
		if invalidOwner(err) {
			return typed(shared.ErrorConflict, "namespace.owner_not_verified")
		}
		return unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, namespaceID shared.UUID) (domain.Namespace, error) {
	if !validUUID(namespaceID) {
		return domain.Namespace{}, typed(shared.ErrorInvalid, "namespace.invalid_id")
	}
	return repository.get(ctx, tenantID, `namespace_id = $2::uuid`, namespaceID.String())
}

func (repository *Repository) GetByPath(ctx context.Context, tenantID shared.UUID, path string) (domain.Namespace, error) {
	if err := domain.ValidatePath(path); err != nil {
		return domain.Namespace{}, typed(shared.ErrorInvalid, "namespace.invalid_path")
	}
	return repository.get(ctx, tenantID, `path = $2`, path)
}

func (repository *Repository) get(ctx context.Context, tenantID shared.UUID, predicate string, argument string) (domain.Namespace, error) {
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Namespace{}, err
	}
	defer rollback(tx)
	row := tx.QueryRow(ctx, namespaceSelect+` WHERE tenant_id = $1::uuid AND `+predicate, tenantID.String(), argument)
	value, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Namespace{}, typed(shared.ErrorNotFound, "namespace.not_found")
	}
	if err != nil {
		return domain.Namespace{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Namespace{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]domain.Namespace, error) {
	if limit < 1 || limit > domain.MaxListSize {
		return nil, typed(shared.ErrorInvalid, "namespace.invalid_page_size")
	}
	if after != nil && !validUUID(*after) {
		return nil, typed(shared.ErrorInvalid, "namespace.invalid_cursor")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	query := namespaceSelect + ` WHERE tenant_id = $1::uuid`
	arguments := []any{tenantID.String()}
	if after != nil {
		query += ` AND namespace_id > $2::uuid`
		arguments = append(arguments, after.String())
	}
	arguments = append(arguments, limit)
	query += ` ORDER BY namespace_id LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.Namespace, 0, limit)
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
		return nil, typed(shared.ErrorInvalid, "namespace.invalid_tenant")
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

const namespaceSelect = `SELECT tenant_id::text, namespace_id::text, path,
       owner_publisher_id::text, created_at
  FROM public.namespaces`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Namespace, error) {
	var tenant, identifier, path, owner string
	var createdAt time.Time
	if err := row.Scan(&tenant, &identifier, &path, &owner, &createdAt); err != nil {
		return domain.Namespace{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.Namespace{}, err
	}
	namespaceID, err := shared.ParseUUID(identifier)
	if err != nil {
		return domain.Namespace{}, err
	}
	ownerID, err := shared.ParseUUID(owner)
	if err != nil {
		return domain.Namespace{}, err
	}
	return domain.Restore(tenantID, namespaceID, path, ownerID, createdAt)
}

func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }

func uniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func invalidOwner(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && (postgresError.Code == "23503" ||
		postgresError.Code == "23514" && postgresError.ConstraintName == "namespaces_verified_owner_check")
}

func typed(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}

func unavailable() error { return typed(shared.ErrorUnavailable, "namespace.persistence_unavailable") }

func validUUID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

var _ domain.Repository = (*Repository)(nil)
