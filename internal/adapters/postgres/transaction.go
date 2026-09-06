package postgres

import (
	"context"
	"fmt"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	transactionport "github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
	"github.com/jackc/pgx/v5"
)

type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type transactionContextKey struct{}

type transactionContext struct {
	tenantID shared.UUID
	tx       pgx.Tx
}

// TransactionManager composes tenant-scoped repository calls in one PostgreSQL
// transaction while keeping pgx types out of the application-facing port.
type TransactionManager struct {
	db Beginner
}

func NewTransactionManager(db Beginner) (*TransactionManager, error) {
	if db == nil {
		return nil, fmt.Errorf("postgres transaction manager: database is required")
	}
	return &TransactionManager{db: db}, nil
}

func (manager *TransactionManager) WithinTransaction(ctx context.Context, tenantID shared.UUID, operation func(context.Context) error) error {
	if ctx == nil || operation == nil || !validTenantID(tenantID) {
		return transactionError(shared.ErrorInvalid, "transaction.invalid_request")
	}
	if current, ok := ctx.Value(transactionContextKey{}).(transactionContext); ok {
		if current.tenantID != tenantID {
			return transactionError(shared.ErrorInvalid, "transaction.tenant_mismatch")
		}
		return withinNested(ctx, current, operation)
	}

	tx, err := manager.db.Begin(ctx)
	if err != nil {
		return transactionUnavailable()
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `SELECT set_config('thinkpixelmp.tenant_id', $1, true)`, tenantID.String()); err != nil {
		return transactionUnavailable()
	}
	transactionCtx := context.WithValue(ctx, transactionContextKey{}, transactionContext{tenantID: tenantID, tx: tx})
	if err := operation(transactionCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return transactionUnavailable()
	}
	return nil
}

func withinNested(ctx context.Context, current transactionContext, operation func(context.Context) error) error {
	tx, err := current.tx.Begin(ctx)
	if err != nil {
		return transactionUnavailable()
	}
	defer rollback(tx)
	nestedCtx := context.WithValue(ctx, transactionContextKey{}, transactionContext{tenantID: current.tenantID, tx: tx})
	if err := operation(nestedCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return transactionUnavailable()
	}
	return nil
}

// BeginRepositoryTransaction starts an independently owned transaction, or a
// savepoint when ctx already carries a transaction for the same tenant.
func BeginRepositoryTransaction(ctx context.Context, db Beginner, tenantID shared.UUID) (pgx.Tx, error) {
	if ctx == nil || db == nil || !validTenantID(tenantID) {
		return nil, transactionError(shared.ErrorInvalid, "transaction.invalid_request")
	}
	if current, ok := ctx.Value(transactionContextKey{}).(transactionContext); ok {
		if current.tenantID != tenantID {
			return nil, transactionError(shared.ErrorInvalid, "transaction.tenant_mismatch")
		}
		tx, err := current.tx.Begin(ctx)
		if err != nil {
			return nil, transactionUnavailable()
		}
		return tx, nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, transactionUnavailable()
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('thinkpixelmp.tenant_id', $1, true)`, tenantID.String()); err != nil {
		rollback(tx)
		return nil, transactionUnavailable()
	}
	return tx, nil
}

// CurrentTransaction returns the adapter transaction carried by ctx for
// transaction-only repository writes such as audit and outbox records.
func CurrentTransaction(ctx context.Context, tenantID shared.UUID) (pgx.Tx, error) {
	if ctx == nil || !validTenantID(tenantID) {
		return nil, transactionError(shared.ErrorInvalid, "transaction.invalid_request")
	}
	current, ok := ctx.Value(transactionContextKey{}).(transactionContext)
	if !ok {
		return nil, transactionError(shared.ErrorInvalid, "transaction.required")
	}
	if current.tenantID != tenantID {
		return nil, transactionError(shared.ErrorInvalid, "transaction.tenant_mismatch")
	}
	return current.tx, nil
}

func validTenantID(id shared.UUID) bool {
	_, err := id.MarshalText()
	return err == nil
}

func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }

func transactionError(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}

func transactionUnavailable() error {
	return transactionError(shared.ErrorUnavailable, "transaction.persistence_unavailable")
}

var _ transactionport.Manager = (*TransactionManager)(nil)
