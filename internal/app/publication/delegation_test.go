package publication_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/outbox"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/security"
)

func TestDelegationCreateRevokeAndReplay(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	rootID := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	rootOwner := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	receiver := parseID(t, "01991e80-2c00-7000-8000-000000000004")
	root, _ := namespace.New(tenant, rootID, "acme", rootOwner, fixedNow)
	namespaces := &namespaceMemory{values: []namespace.Namespace{root}}
	delegations := &delegationMemory{verified: map[shared.UUID]bool{rootOwner: true, receiver: true}}
	service := newDelegationService(t, namespaces, delegations, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin,
	}})
	actor := identity.Identity{TenantID: tenant, PrincipalID: "admin"}
	command := publication.CreateDelegation{NamespaceID: rootID, ChildPrefix: "acme/security", PublisherID: receiver}
	created, err := service.Create(context.Background(), actor, "delegate-1", command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Create(context.Background(), actor, "delegate-1", command)
	if err != nil || replayed.ID() != created.ID() || delegations.creates != 1 || len(delegations.events.messages) != 1 {
		t.Fatalf("create replay: %#v creates=%d %v", replayed, delegations.creates, err)
	}
	service = newDelegationService(t, namespaces, delegations, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin,
	}})
	revoked, err := service.Revoke(context.Background(), actor, "revoke-1", publication.RevokeDelegation{
		DelegationID: created.ID(), ExpectedVersion: 1, ReasonCode: "ownership.changed", Explanation: "team changed",
	})
	if err != nil || revoked.State() != namespace.DelegationRevoked || revoked.StateVersion() != 2 {
		t.Fatalf("revoke: %#v %v", revoked, err)
	}
	replayedRevoke, err := service.Revoke(context.Background(), actor, "revoke-1", publication.RevokeDelegation{
		DelegationID: created.ID(), ExpectedVersion: 1, ReasonCode: "ownership.changed", Explanation: "team changed",
	})
	if err != nil || replayedRevoke.State() != namespace.DelegationRevoked || delegations.revokes != 1 || len(delegations.events.messages) != 2 {
		t.Fatalf("revoke replay: %#v revokes=%d %v", replayedRevoke, delegations.revokes, err)
	}
	for _, message := range delegations.events.messages {
		if strings.Contains(string(message.Payload()), "acme/security") || strings.Contains(string(message.Payload()), "team changed") {
			t.Fatal("delegation event exposed path or free-form explanation")
		}
	}
}

func TestDelegationMutationRequiresExactNamespaceRole(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	rootID := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	root, _ := namespace.New(tenant, rootID, "acme", owner, fixedNow)
	delegations := &delegationMemory{verified: map[shared.UUID]bool{owner: true}}
	service := newDelegationService(t, &namespaceMemory{values: []namespace.Namespace{root}}, delegations, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin,
	}})
	_, err := service.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "publisher"}, "delegate-1",
		publication.CreateDelegation{NamespaceID: rootID, ChildPrefix: "acme/security", PublisherID: owner})
	assertTyped(t, err, shared.ErrorForbidden, "authorization.denied")
	if delegations.creates != 0 || len(delegations.events.messages) != 0 {
		t.Fatal("unauthorized delegation mutation reached persistence")
	}
}

func TestPublicationAuthorityRequiresExactRoleAndLongestPrefixOwner(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	rootOwner := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	receiver := parseID(t, "01991e80-2c00-7000-8000-000000000004")
	delegations := &delegationMemory{resolved: map[string]shared.UUID{"acme": rootOwner, "acme/security": receiver}}
	service := newDelegationService(t, &namespaceMemory{}, delegations, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin,
	}})
	actor := identity.Identity{TenantID: tenant, PrincipalID: "publisher"}
	if err := service.AuthorizePublication(context.Background(), actor, receiver, "acme/security/reviewer"); err != nil {
		t.Fatal(err)
	}
	assertTyped(t, service.AuthorizePublication(context.Background(), actor, rootOwner, "acme/security/reviewer"), shared.ErrorForbidden, "namespace.publication_not_authorized")
	assertTyped(t, service.AuthorizePublication(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "reader"}, receiver, "acme/security/reviewer"), shared.ErrorForbidden, "authorization.denied")
}

func newDelegationService(t *testing.T, namespaces *namespaceMemory, delegations *delegationMemory, grants []authorization.Grant) *publication.DelegationService {
	t.Helper()
	authorizer, err := security.NewAuthorizer(grants)
	if err != nil {
		t.Fatal(err)
	}
	ids, _ := shared.NewUUIDGenerator(clock.Fixed{Time: fixedNow}, bytes.NewReader(bytes.Repeat([]byte{31}, 1000)))
	service, err := publication.NewDelegationService(publication.DelegationDependencies{
		Namespaces: namespaces, Delegations: delegations, Idempotency: &idempotencyMemory{},
		Transactions: &transactionMemory{}, Authorizer: authorizer, IDs: ids, Clock: clock.Fixed{Time: fixedNow},
		Outbox: &delegations.events, EventSource: "urn:thinkpixel:mp:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type delegationMemory struct {
	values   []namespace.Delegation
	verified map[shared.UUID]bool
	resolved map[string]shared.UUID
	creates  int
	revokes  int
	events   outboxMemory
}

type outboxMemory struct {
	sequence uint64
	messages []outbox.Message
}

func (memory *outboxMemory) NextSequence(context.Context, shared.UUID) (uint64, error) {
	memory.sequence++
	return memory.sequence, nil
}

func (memory *outboxMemory) Record(_ context.Context, message outbox.Message) error {
	memory.messages = append(memory.messages, message)
	return nil
}

func (memory *delegationMemory) CreateDelegation(_ context.Context, value namespace.Delegation) error {
	if memory.verified != nil && !memory.verified[value.PublisherID()] {
		return typed(shared.ErrorConflict, "namespace.invalid_delegation")
	}
	for _, stored := range memory.values {
		if stored.TenantID() == value.TenantID() && stored.ChildPrefix() == value.ChildPrefix() && stored.State() == namespace.DelegationActive {
			return typed(shared.ErrorConflict, "namespace.delegation_conflict")
		}
	}
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}

func (memory *delegationMemory) GetDelegation(_ context.Context, tenantID, delegationID shared.UUID) (namespace.Delegation, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == delegationID {
			return value, nil
		}
	}
	return namespace.Delegation{}, typed(shared.ErrorNotFound, "namespace.delegation_not_found")
}

func (memory *delegationMemory) RevokeDelegation(_ context.Context, tenantID, delegationID shared.UUID, expected int64, reason shared.ReasonCode, explanation string, at time.Time) (namespace.Delegation, error) {
	for index, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == delegationID {
			if value.StateVersion() != expected {
				return namespace.Delegation{}, typed(shared.ErrorConflict, "namespace.stale_delegation_version")
			}
			updated, err := value.Revoke(reason, explanation, at)
			if err != nil {
				return namespace.Delegation{}, err
			}
			memory.values[index] = updated
			memory.revokes++
			return updated, nil
		}
	}
	return namespace.Delegation{}, errors.New("missing")
}

func (memory *delegationMemory) ResolveOwner(_ context.Context, tenantID shared.UUID, path string) (shared.UUID, error) {
	var owner shared.UUID
	longest := -1
	for prefix, candidate := range memory.resolved {
		if namespace.IsStrictDescendant(prefix, path) || prefix == path {
			if len(prefix) > longest {
				longest, owner = len(prefix), candidate
			}
		}
	}
	if longest < 0 {
		return shared.UUID{}, typed(shared.ErrorForbidden, "namespace.publication_not_authorized")
	}
	return owner, nil
}
