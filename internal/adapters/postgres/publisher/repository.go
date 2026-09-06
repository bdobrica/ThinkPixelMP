// Package publisher persists tenant-scoped Publisher aggregates in PostgreSQL.
package publisher

import (
	"context"
	"errors"
	"fmt"
	"time"

	postgres "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres"
	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	domain "github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
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
		return nil, fmt.Errorf("publisher repository: database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) Create(ctx context.Context, value domain.Publisher) error {
	tx, err := repository.begin(ctx, value.TenantID())
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `INSERT INTO public.publishers
  (tenant_id, publisher_id, slug, display_name, description, current_state_version, created_at)
 VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), NULLIF($5, ''), 1, $6)`,
		value.TenantID().String(), value.ID().String(), value.Slug(), value.DisplayName(), value.Description(), value.CreatedAt())
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.publisher_state_records
   (tenant_id, publisher_id, version, state, recorded_at)
  VALUES ($1::uuid, $2::uuid, 1, 'claimed', $3)`,
			value.TenantID().String(), value.ID().String(), value.CreatedAt())
	}
	if err != nil {
		if uniqueViolation(err) {
			return typed(shared.ErrorConflict, "publisher.conflict")
		}
		return unavailable()
	}
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: value.TenantID(), Action: postgresaudit.ActionPublisherCreated,
		ResourceType: "publisher", ResourceID: value.ID().String(),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable()
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, tenantID, publisherID shared.UUID) (domain.Publisher, error) {
	if !validUUID(publisherID) {
		return domain.Publisher{}, typed(shared.ErrorInvalid, "publisher.invalid_id")
	}
	return repository.get(ctx, tenantID, `p.publisher_id = $2::uuid`, publisherID.String())
}

func (repository *Repository) GetBySlug(ctx context.Context, tenantID shared.UUID, slug string) (domain.Publisher, error) {
	if err := domain.ValidateSlug(slug); err != nil {
		return domain.Publisher{}, typed(shared.ErrorInvalid, "publisher.invalid_slug")
	}
	return repository.get(ctx, tenantID, `p.slug = $2`, slug)
}

func (repository *Repository) get(ctx context.Context, tenantID shared.UUID, predicate string, argument string) (domain.Publisher, error) {
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Publisher{}, err
	}
	defer rollback(tx)
	row := tx.QueryRow(ctx, publisherSelect+` WHERE p.tenant_id = $1::uuid AND `+predicate,
		tenantID.String(), argument)
	value, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Publisher{}, typed(shared.ErrorNotFound, "publisher.not_found")
	}
	if err != nil {
		return domain.Publisher{}, unavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Publisher{}, unavailable()
	}
	return value, nil
}

func (repository *Repository) List(ctx context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]domain.Publisher, error) {
	if limit < 1 || limit > domain.MaxListSize {
		return nil, typed(shared.ErrorInvalid, "publisher.invalid_page_size")
	}
	if after != nil && !validUUID(*after) {
		return nil, typed(shared.ErrorInvalid, "publisher.invalid_cursor")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	query := publisherSelect + ` WHERE p.tenant_id = $1::uuid`
	arguments := []any{tenantID.String()}
	if after != nil {
		query += ` AND p.publisher_id > $2::uuid`
		arguments = append(arguments, after.String())
	}
	arguments = append(arguments, limit)
	query += ` ORDER BY p.publisher_id LIMIT $` + fmt.Sprint(len(arguments))
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	values := make([]domain.Publisher, 0, limit)
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

func (repository *Repository) ChangeState(ctx context.Context, tenantID, publisherID shared.UUID, state domain.State, reason shared.ReasonCode, explanation string, at time.Time) (domain.Publisher, error) {
	if !validUUID(publisherID) {
		return domain.Publisher{}, typed(shared.ErrorInvalid, "publisher.invalid_id")
	}
	tx, err := repository.begin(ctx, tenantID)
	if err != nil {
		return domain.Publisher{}, err
	}
	defer rollback(tx)
	row := tx.QueryRow(ctx, publisherSelect+` WHERE p.tenant_id = $1::uuid AND p.publisher_id = $2::uuid FOR UPDATE OF p`,
		tenantID.String(), publisherID.String())
	current, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Publisher{}, typed(shared.ErrorNotFound, "publisher.not_found")
	}
	if err != nil {
		return domain.Publisher{}, unavailable()
	}
	next, record, err := current.Transition(state, reason, explanation, at)
	if errors.Is(err, domain.ErrInvalidTransition) {
		return domain.Publisher{}, typed(shared.ErrorConflict, "publisher.invalid_transition")
	}
	if err != nil {
		return domain.Publisher{}, typed(shared.ErrorInvalid, "publisher.invalid_transition_metadata")
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.publisher_state_records
  (tenant_id, publisher_id, version, state, reason_code, explanation, recorded_at)
 VALUES ($1::uuid, $2::uuid, $3, $4, $5, NULLIF($6, ''), $7)`, tenantID.String(), publisherID.String(),
		record.Version, string(record.State), record.Reason.String(), record.Explanation, record.RecordedAt)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE public.publishers SET current_state_version = $3
 WHERE tenant_id = $1::uuid AND publisher_id = $2::uuid`, tenantID.String(), publisherID.String(), record.Version)
	}
	if err != nil {
		return domain.Publisher{}, unavailable()
	}
	decision, _ := shared.NewReasonCode(string(state))
	if err := postgresaudit.Record(ctx, tx, postgresaudit.RecordParams{
		TenantID: tenantID, Action: postgresaudit.ActionPublisherStateChanged,
		ResourceType: "publisher", ResourceID: publisherID.String(), Decision: &decision, ReasonCodes: []shared.ReasonCode{reason},
	}); err != nil {
		return domain.Publisher{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Publisher{}, unavailable()
	}
	return next, nil
}

func (repository *Repository) begin(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if _, err := tenantID.MarshalText(); err != nil {
		return nil, typed(shared.ErrorInvalid, "publisher.invalid_tenant")
	}
	return postgres.BeginRepositoryTransaction(ctx, repository.db, tenantID)
}

const publisherSelect = `SELECT p.tenant_id::text, p.publisher_id::text, p.slug,
       COALESCE(p.display_name, ''), COALESCE(p.description, ''),
       s.state, s.version, p.created_at
  FROM public.publishers p
  JOIN public.publisher_state_records s
    ON s.tenant_id = p.tenant_id
   AND s.publisher_id = p.publisher_id
   AND s.version = p.current_state_version`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Publisher, error) {
	var tenant, identifier, slug, displayName, description, state string
	var version int64
	var createdAt time.Time
	if err := row.Scan(&tenant, &identifier, &slug, &displayName, &description, &state, &version, &createdAt); err != nil {
		return domain.Publisher{}, err
	}
	tenantID, err := shared.ParseUUID(tenant)
	if err != nil {
		return domain.Publisher{}, err
	}
	publisherID, err := shared.ParseUUID(identifier)
	if err != nil {
		return domain.Publisher{}, err
	}
	return domain.Restore(tenantID, publisherID, slug, displayName, description, domain.State(state), version, createdAt)
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

func unavailable() error { return typed(shared.ErrorUnavailable, "publisher.persistence_unavailable") }

func validUUID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

var _ domain.Repository = (*Repository)(nil)
