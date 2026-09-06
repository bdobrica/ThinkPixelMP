package publication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
)

const (
	DefaultNamespacePageSize = 50
	MaxNamespacePageSize     = 200
	namespaceCursorLifetime  = 15 * time.Minute
)

var (
	createNamespaceAction = mustReasonCode("namespace.create")
	namespaceResourceType = mustReasonCode("namespace")
	namespaceQueryDigest  = shared.SHA256Digest([]byte(`{"sort":"namespace_id"}`))
)

// NamespaceService coordinates authenticated, tenant-scoped Namespace use cases.
type NamespaceService struct {
	namespaces   namespace.Repository
	idempotency  idempotency.Repository
	transactions transaction.Manager
	authorizer   authorization.Authorizer
	ids          IdentifierGenerator
	clock        clock.Clock
	cursors      *shared.CursorCodec
}

type NamespaceDependencies struct {
	Namespaces   namespace.Repository
	Idempotency  idempotency.Repository
	Transactions transaction.Manager
	Authorizer   authorization.Authorizer
	IDs          IdentifierGenerator
	Clock        clock.Clock
	Cursors      *shared.CursorCodec
}

func NewNamespaceService(dependencies NamespaceDependencies) (*NamespaceService, error) {
	if dependencies.Namespaces == nil || dependencies.Idempotency == nil || dependencies.Transactions == nil ||
		dependencies.Authorizer == nil || dependencies.IDs == nil || dependencies.Clock == nil || dependencies.Cursors == nil {
		return nil, errors.New("namespace service: all dependencies are required")
	}
	return &NamespaceService{
		namespaces: dependencies.Namespaces, idempotency: dependencies.Idempotency,
		transactions: dependencies.Transactions, authorizer: dependencies.Authorizer,
		ids: dependencies.IDs, clock: dependencies.Clock, cursors: dependencies.Cursors,
	}, nil
}

type CreateNamespace struct {
	Path             string      `json:"path"`
	OwnerPublisherID shared.UUID `json:"owner_publisher_id"`
}

// Create authorizes namespace administration and atomically couples the
// idempotency claim, Namespace/audit write, and established response identity.
func (service *NamespaceService) Create(ctx context.Context, actor identity.Identity, key string, command CreateNamespace) (namespace.Namespace, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionManageNamespaces); err != nil {
		return namespace.Namespace{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return namespace.Namespace{}, err
	}
	if err := namespace.ValidatePath(command.Path); err != nil {
		return namespace.Namespace{}, typed(shared.ErrorInvalid, "namespace.invalid_request")
	}
	if _, err := command.OwnerPublisherID.MarshalText(); err != nil {
		return namespace.Namespace{}, typed(shared.ErrorInvalid, "namespace.invalid_request")
	}
	canonical, err := json.Marshal(command)
	if err != nil {
		return namespace.Namespace{}, typed(shared.ErrorInternal, "namespace.request_digest_unavailable")
	}
	digest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, createNamespaceAction, key); err != nil {
		return namespace.Namespace{}, typed(shared.ErrorInvalid, "namespace.invalid_idempotency_key")
	}

	var result namespace.Namespace
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		recordID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "namespace.identifier_unavailable")
		}
		record, err := idempotency.New(actor.TenantID, recordID, actor.PrincipalID, createNamespaceAction, key,
			digest, now, now.Add(idempotency.MinimumRetention))
		if err != nil {
			return typed(shared.ErrorInvalid, "namespace.invalid_request")
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replayNamespace(transactionContext, actor.TenantID, stored, &result)
		}

		namespaceID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "namespace.identifier_unavailable")
		}
		result, err = namespace.New(actor.TenantID, namespaceID, command.Path, command.OwnerPublisherID, now)
		if err != nil {
			return typed(shared.ErrorInvalid, "namespace.invalid_request")
		}
		if err := service.namespaces.Create(transactionContext, result); err != nil {
			return err
		}
		established, err := idempotency.NewResult(http.StatusCreated, namespaceResourceType.String(), result.ID().String())
		if err != nil {
			return typed(shared.ErrorInternal, "namespace.result_unavailable")
		}
		_, err = service.idempotency.Complete(transactionContext, actor.TenantID, actor.PrincipalID,
			createNamespaceAction, key, digest, established, now)
		return err
	})
	return result, err
}

func (service *NamespaceService) replayNamespace(ctx context.Context, tenantID shared.UUID, record idempotency.Record, destination *namespace.Namespace) error {
	established, ok := record.Result()
	if !ok {
		return typed(shared.ErrorConflict, "idempotency.in_progress")
	}
	resourceType, hasType := established.ResourceType()
	resourceID, hasID := established.ResourceID()
	if established.Status() != http.StatusCreated || !hasType || resourceType != namespaceResourceType || !hasID {
		return typed(shared.ErrorConflict, "idempotency.result_mismatch")
	}
	identifier, err := shared.ParseUUID(resourceID)
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.result_unavailable")
	}
	*destination, err = service.namespaces.Get(ctx, tenantID, identifier)
	return err
}

func (service *NamespaceService) Get(ctx context.Context, actor identity.Identity, namespaceID shared.UUID) (namespace.Namespace, error) {
	if err := validateIdentity(actor); err != nil {
		return namespace.Namespace{}, err
	}
	return service.namespaces.Get(ctx, actor.TenantID, namespaceID)
}

type NamespacePage struct {
	Items      []namespace.Namespace
	NextCursor string
}

func (service *NamespaceService) List(ctx context.Context, actor identity.Identity, pageSize int, encodedCursor string) (NamespacePage, error) {
	if err := validateIdentity(actor); err != nil {
		return NamespacePage{}, err
	}
	if pageSize < 1 || pageSize > MaxNamespacePageSize {
		return NamespacePage{}, typed(shared.ErrorInvalid, "namespace.invalid_page_size")
	}
	scope := shared.CursorScope{TenantID: actor.TenantID, Endpoint: "/v1/namespaces", QueryDigest: namespaceQueryDigest, PageSize: pageSize}
	var after *shared.UUID
	if encodedCursor != "" {
		cursor, err := service.cursors.Decode(encodedCursor, scope)
		if err != nil {
			return NamespacePage{}, typed(shared.ErrorInvalid, "namespace.invalid_cursor")
		}
		position, err := shared.ParseUUID(cursor.Position)
		if err != nil {
			return NamespacePage{}, typed(shared.ErrorInvalid, "namespace.invalid_cursor")
		}
		after = &position
	}
	values, err := service.namespaces.List(ctx, actor.TenantID, after, pageSize+1)
	if err != nil {
		return NamespacePage{}, err
	}
	page := NamespacePage{Items: values}
	if len(values) > pageSize {
		page.Items = values[:pageSize]
		page.NextCursor, err = service.cursors.Encode(scope, shared.Cursor{
			Position: page.Items[len(page.Items)-1].ID().String(), ExpiresAt: service.clock.Now().Add(namespaceCursorLifetime),
		})
		if err != nil {
			return NamespacePage{}, typed(shared.ErrorInternal, "namespace.cursor_unavailable")
		}
	}
	return page, nil
}
