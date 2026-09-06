package publication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/outbox"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
)

var (
	createDelegationAction = mustReasonCode("namespace.delegate")
	revokeDelegationAction = mustReasonCode("namespace.delegation_revoke")
	delegationResourceType = mustReasonCode("namespace_delegation")
)

type DelegationService struct {
	namespaces   namespace.Repository
	delegations  namespace.DelegationRepository
	idempotency  idempotency.Repository
	transactions transaction.Manager
	authorizer   authorization.Authorizer
	ids          IdentifierGenerator
	clock        clock.Clock
	outbox       outbox.Writer
	eventSource  string
}

type DelegationDependencies struct {
	Namespaces   namespace.Repository
	Delegations  namespace.DelegationRepository
	Idempotency  idempotency.Repository
	Transactions transaction.Manager
	Authorizer   authorization.Authorizer
	IDs          IdentifierGenerator
	Clock        clock.Clock
	Outbox       outbox.Writer
	EventSource  string
}

func NewDelegationService(dependencies DelegationDependencies) (*DelegationService, error) {
	if dependencies.Namespaces == nil || dependencies.Delegations == nil || dependencies.Idempotency == nil ||
		dependencies.Transactions == nil || dependencies.Authorizer == nil || dependencies.IDs == nil || dependencies.Clock == nil ||
		dependencies.Outbox == nil || dependencies.EventSource == "" {
		return nil, errors.New("delegation service: all dependencies are required")
	}
	return &DelegationService{dependencies.Namespaces, dependencies.Delegations, dependencies.Idempotency,
		dependencies.Transactions, dependencies.Authorizer, dependencies.IDs, dependencies.Clock, dependencies.Outbox, dependencies.EventSource}, nil
}

type CreateDelegation struct {
	NamespaceID shared.UUID `json:"namespace_id"`
	ChildPrefix string      `json:"child_prefix"`
	PublisherID shared.UUID `json:"publisher_id"`
}

func (service *DelegationService) Create(ctx context.Context, actor identity.Identity, key string, command CreateDelegation) (namespace.Delegation, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionManageNamespaces); err != nil {
		return namespace.Delegation{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return namespace.Delegation{}, err
	}
	if _, err := command.NamespaceID.MarshalText(); err != nil || namespace.ValidatePath(command.ChildPrefix) != nil {
		return namespace.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_request")
	}
	if _, err := command.PublisherID.MarshalText(); err != nil {
		return namespace.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_request")
	}
	canonical, _ := json.Marshal(command)
	digest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, createDelegationAction, key); err != nil {
		return namespace.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_idempotency_key")
	}
	var result namespace.Delegation
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		record, err := service.newRecord(actor, createDelegationAction, key, digest, now)
		if err != nil {
			return err
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replay(transactionContext, actor.TenantID, stored, http.StatusCreated, &result)
		}
		root, err := service.namespaces.Get(transactionContext, actor.TenantID, command.NamespaceID)
		if err != nil {
			return err
		}
		identifier, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "namespace.identifier_unavailable")
		}
		result, err = namespace.NewDelegation(actor.TenantID, identifier, root.ID(), root.Path(), command.PublisherID, command.ChildPrefix, now)
		if err != nil {
			return typed(shared.ErrorInvalid, "namespace.invalid_delegation_request")
		}
		if err := service.delegations.CreateDelegation(transactionContext, result); err != nil {
			return err
		}
		if err := service.recordEvent(transactionContext, result, "", "active", "", now); err != nil {
			return err
		}
		return service.complete(transactionContext, actor, createDelegationAction, key, digest, result.ID(), http.StatusCreated, now)
	})
	return result, err
}

type RevokeDelegation struct {
	DelegationID    shared.UUID `json:"delegation_id"`
	ExpectedVersion int64       `json:"expected_version"`
	ReasonCode      string      `json:"reason_code"`
	Explanation     string      `json:"explanation,omitempty"`
}

func (service *DelegationService) Revoke(ctx context.Context, actor identity.Identity, key string, command RevokeDelegation) (namespace.Delegation, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionManageNamespaces); err != nil {
		return namespace.Delegation{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return namespace.Delegation{}, err
	}
	reason, reasonErr := shared.NewReasonCode(command.ReasonCode)
	if _, err := command.DelegationID.MarshalText(); err != nil || command.ExpectedVersion < 1 || reasonErr != nil {
		return namespace.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_revocation")
	}
	canonical, _ := json.Marshal(command)
	digest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, revokeDelegationAction, key); err != nil {
		return namespace.Delegation{}, typed(shared.ErrorInvalid, "namespace.invalid_idempotency_key")
	}
	var result namespace.Delegation
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		record, err := service.newRecord(actor, revokeDelegationAction, key, digest, now)
		if err != nil {
			return err
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replay(transactionContext, actor.TenantID, stored, http.StatusOK, &result)
		}
		result, err = service.delegations.RevokeDelegation(transactionContext, actor.TenantID, command.DelegationID,
			command.ExpectedVersion, reason, command.Explanation, now)
		if err != nil {
			return err
		}
		if err := service.recordEvent(transactionContext, result, "active", "revoked", reason.String(), now); err != nil {
			return err
		}
		return service.complete(transactionContext, actor, revokeDelegationAction, key, digest, result.ID(), http.StatusOK, now)
	})
	return result, err
}

func (service *DelegationService) recordEvent(ctx context.Context, delegation namespace.Delegation, previousState, currentState, reason string, now time.Time) error {
	sequence, err := service.outbox.NextSequence(ctx, delegation.TenantID())
	if err != nil {
		return err
	}
	eventID, err := service.ids.New()
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.identifier_unavailable")
	}
	eventType := "io.thinkpixel.mp.namespace.delegated.v1"
	data := map[string]any{
		"tenant_id": delegation.TenantID().String(), "transaction_cursor": strconv.FormatUint(sequence, 10),
		"namespace_id": delegation.NamespaceID().String(), "delegation_id": delegation.ID().String(),
		"publisher_id": delegation.PublisherID().String(), "current_state": currentState,
	}
	if previousState != "" {
		eventType = "io.thinkpixel.mp.namespace.delegation-revoked.v1"
		data["previous_state"], data["reason_code"] = previousState, reason
	}
	payload, err := json.Marshal(map[string]any{
		"specversion": "1.0", "id": eventID.String(), "source": service.eventSource, "type": eventType,
		"subject": delegation.ID().String(), "time": now.UTC(), "datacontenttype": outbox.DataContentType,
		"sequence": sequence, "data": data,
	})
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.event_unavailable")
	}
	message, err := outbox.New(delegation.TenantID(), eventID, sequence, service.eventSource, eventType, delegation.ID().String(), payload, now)
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.event_unavailable")
	}
	return service.outbox.Record(ctx, message)
}

// AuthorizePublication applies both the exact publication role and the live,
// longest-prefix Publisher ownership rule. Neither check grants runtime authority.
func (service *DelegationService) AuthorizePublication(ctx context.Context, actor identity.Identity, publisherID shared.UUID, path string) error {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionPublish); err != nil {
		return err
	}
	if _, err := publisherID.MarshalText(); err != nil || namespace.ValidatePath(path) != nil {
		return typed(shared.ErrorInvalid, "namespace.invalid_publication_request")
	}
	owner, err := service.delegations.ResolveOwner(ctx, actor.TenantID, path)
	if err != nil {
		return err
	}
	if owner != publisherID {
		return typed(shared.ErrorForbidden, "namespace.publication_not_authorized")
	}
	return nil
}

func (service *DelegationService) newRecord(actor identity.Identity, action shared.ReasonCode, key string, digest shared.Digest, now time.Time) (idempotency.Record, error) {
	identifier, err := service.ids.New()
	if err != nil {
		return idempotency.Record{}, typed(shared.ErrorInternal, "namespace.identifier_unavailable")
	}
	record, err := idempotency.New(actor.TenantID, identifier, actor.PrincipalID, action, key, digest, now, now.Add(idempotency.MinimumRetention))
	if err != nil {
		return idempotency.Record{}, typed(shared.ErrorInvalid, "namespace.invalid_delegation_request")
	}
	return record, nil
}

func (service *DelegationService) complete(ctx context.Context, actor identity.Identity, action shared.ReasonCode, key string, digest shared.Digest, id shared.UUID, status uint16, now time.Time) error {
	result, err := idempotency.NewResult(status, delegationResourceType.String(), id.String())
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.result_unavailable")
	}
	_, err = service.idempotency.Complete(ctx, actor.TenantID, actor.PrincipalID, action, key, digest, result, now)
	return err
}

func (service *DelegationService) replay(ctx context.Context, tenantID shared.UUID, record idempotency.Record, status uint16, destination *namespace.Delegation) error {
	result, ok := record.Result()
	if !ok {
		return typed(shared.ErrorConflict, "idempotency.in_progress")
	}
	resourceType, hasType := result.ResourceType()
	resourceID, hasID := result.ResourceID()
	if result.Status() != status || !hasType || resourceType != delegationResourceType || !hasID {
		return typed(shared.ErrorConflict, "idempotency.result_mismatch")
	}
	identifier, err := shared.ParseUUID(resourceID)
	if err != nil {
		return typed(shared.ErrorInternal, "namespace.result_unavailable")
	}
	*destination, err = service.delegations.GetDelegation(ctx, tenantID, identifier)
	return err
}
