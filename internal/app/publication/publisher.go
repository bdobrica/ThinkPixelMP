package publication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
)

const (
	DefaultPublisherPageSize = 50
	MaxPublisherPageSize     = 200
	publisherCursorLifetime  = 15 * time.Minute
)

var (
	createPublisherAction = mustReasonCode("publisher.create")
	publisherResourceType = mustReasonCode("publisher")
	publisherQueryDigest  = shared.SHA256Digest([]byte(`{"sort":"publisher_id"}`))
)

// IdentifierGenerator supplies server-owned UUIDv7 resource identifiers.
type IdentifierGenerator interface{ New() (shared.UUID, error) }

// PublisherService coordinates authenticated, tenant-scoped Publisher use cases.
type PublisherService struct {
	publishers   publisher.Repository
	idempotency  idempotency.Repository
	transactions transaction.Manager
	authorizer   authorization.Authorizer
	ids          IdentifierGenerator
	clock        clock.Clock
	cursors      *shared.CursorCodec
}

type PublisherDependencies struct {
	Publishers   publisher.Repository
	Idempotency  idempotency.Repository
	Transactions transaction.Manager
	Authorizer   authorization.Authorizer
	IDs          IdentifierGenerator
	Clock        clock.Clock
	Cursors      *shared.CursorCodec
}

func NewPublisherService(dependencies PublisherDependencies) (*PublisherService, error) {
	if dependencies.Publishers == nil || dependencies.Idempotency == nil || dependencies.Transactions == nil ||
		dependencies.Authorizer == nil || dependencies.IDs == nil || dependencies.Clock == nil || dependencies.Cursors == nil {
		return nil, errors.New("publisher service: all dependencies are required")
	}
	return &PublisherService{
		publishers: dependencies.Publishers, idempotency: dependencies.Idempotency,
		transactions: dependencies.Transactions, authorizer: dependencies.Authorizer,
		ids: dependencies.IDs, clock: dependencies.Clock, cursors: dependencies.Cursors,
	}, nil
}

type CreatePublisher struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Create authorizes publisher administration and atomically couples the
// idempotency claim, Publisher/audit write, and established response identity.
func (service *PublisherService) Create(ctx context.Context, actor identity.Identity, key string, command CreatePublisher) (publisher.Publisher, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionManagePublishers); err != nil {
		return publisher.Publisher{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return publisher.Publisher{}, err
	}
	canonical, err := json.Marshal(command)
	if err != nil {
		return publisher.Publisher{}, typed(shared.ErrorInternal, "publisher.request_digest_unavailable")
	}
	digest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, createPublisherAction, key); err != nil {
		return publisher.Publisher{}, typed(shared.ErrorInvalid, "publisher.invalid_idempotency_key")
	}

	var result publisher.Publisher
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		recordID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "publisher.identifier_unavailable")
		}
		record, err := idempotency.New(actor.TenantID, recordID, actor.PrincipalID, createPublisherAction, key,
			digest, now, now.Add(idempotency.MinimumRetention))
		if err != nil {
			return typed(shared.ErrorInvalid, "publisher.invalid_request")
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replay(transactionContext, actor.TenantID, stored, &result)
		}

		publisherID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "publisher.identifier_unavailable")
		}
		result, err = publisher.New(actor.TenantID, publisherID, command.Slug, command.DisplayName, command.Description, now)
		if err != nil {
			return typed(shared.ErrorInvalid, "publisher.invalid_request")
		}
		if err := service.publishers.Create(transactionContext, result); err != nil {
			return err
		}
		established, err := idempotency.NewResult(http.StatusCreated, publisherResourceType.String(), result.ID().String())
		if err != nil {
			return typed(shared.ErrorInternal, "publisher.result_unavailable")
		}
		_, err = service.idempotency.Complete(transactionContext, actor.TenantID, actor.PrincipalID,
			createPublisherAction, key, digest, established, now)
		return err
	})
	return result, err
}

func authenticatedAuditContext(ctx context.Context, actor identity.Identity) (context.Context, error) {
	var requestID *shared.UUID
	traceID := ""
	if correlation, ok := audit.ActorFromContext(ctx); ok {
		if identifier, present := correlation.RequestID(); present {
			requestID = &identifier
		}
		traceID = correlation.TraceID()
	}
	verified, err := audit.NewActor(actor.PrincipalID, requestID, traceID)
	if err != nil {
		return nil, typed(shared.ErrorUnauthorized, "authorization.identity_required")
	}
	return audit.WithActor(ctx, verified), nil
}

func (service *PublisherService) replay(ctx context.Context, tenantID shared.UUID, record idempotency.Record, destination *publisher.Publisher) error {
	established, ok := record.Result()
	if !ok {
		return typed(shared.ErrorConflict, "idempotency.in_progress")
	}
	resourceType, hasType := established.ResourceType()
	resourceID, hasID := established.ResourceID()
	if established.Status() != http.StatusCreated || !hasType || resourceType != publisherResourceType || !hasID {
		return typed(shared.ErrorConflict, "idempotency.result_mismatch")
	}
	identifier, err := shared.ParseUUID(resourceID)
	if err != nil {
		return typed(shared.ErrorInternal, "publisher.result_unavailable")
	}
	*destination, err = service.publishers.Get(ctx, tenantID, identifier)
	return err
}

func (service *PublisherService) Get(ctx context.Context, actor identity.Identity, publisherID shared.UUID) (publisher.Publisher, error) {
	if err := validateIdentity(actor); err != nil {
		return publisher.Publisher{}, err
	}
	return service.publishers.Get(ctx, actor.TenantID, publisherID)
}

type PublisherPage struct {
	Items      []publisher.Publisher
	NextCursor string
}

func (service *PublisherService) List(ctx context.Context, actor identity.Identity, pageSize int, encodedCursor string) (PublisherPage, error) {
	if err := validateIdentity(actor); err != nil {
		return PublisherPage{}, err
	}
	if pageSize < 1 || pageSize > MaxPublisherPageSize {
		return PublisherPage{}, typed(shared.ErrorInvalid, "publisher.invalid_page_size")
	}
	scope := shared.CursorScope{TenantID: actor.TenantID, Endpoint: "/v1/publishers", QueryDigest: publisherQueryDigest, PageSize: pageSize}
	var after *shared.UUID
	if encodedCursor != "" {
		cursor, err := service.cursors.Decode(encodedCursor, scope)
		if err != nil {
			return PublisherPage{}, typed(shared.ErrorInvalid, "publisher.invalid_cursor")
		}
		position, err := shared.ParseUUID(cursor.Position)
		if err != nil {
			return PublisherPage{}, typed(shared.ErrorInvalid, "publisher.invalid_cursor")
		}
		after = &position
	}
	values, err := service.publishers.List(ctx, actor.TenantID, after, pageSize+1)
	if err != nil {
		return PublisherPage{}, err
	}
	page := PublisherPage{Items: values}
	if len(values) > pageSize {
		page.Items = values[:pageSize]
		page.NextCursor, err = service.cursors.Encode(scope, shared.Cursor{
			Position: page.Items[len(page.Items)-1].ID().String(), ExpiresAt: service.clock.Now().Add(publisherCursorLifetime),
		})
		if err != nil {
			return PublisherPage{}, typed(shared.ErrorInternal, "publisher.cursor_unavailable")
		}
	}
	return page, nil
}

func validateIdentity(actor identity.Identity) error {
	if _, err := actor.TenantID.MarshalText(); err != nil || actor.PrincipalID == "" {
		return typed(shared.ErrorUnauthorized, "authorization.identity_required")
	}
	return nil
}

func typed(class shared.ErrorClass, value string) error {
	return shared.NewTypedError(class, mustReasonCode(value))
}

func mustReasonCode(value string) shared.ReasonCode {
	code, err := shared.NewReasonCode(value)
	if err != nil {
		panic(err)
	}
	return code
}
