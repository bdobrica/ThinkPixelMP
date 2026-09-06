package publication_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/security"
)

func TestNamespaceCreateIsAuthorizedAtomicAndReplaySafe(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	actor := identity.Identity{TenantID: tenant, PrincipalID: "principal-1"}
	repository := &namespaceMemory{verifiedOwners: map[shared.UUID]bool{owner: true}}
	records := &idempotencyMemory{}
	transactions := &transactionMemory{}
	service := newNamespaceService(t, repository, records, transactions, []authorization.Grant{{
		TenantID: tenant, PrincipalID: actor.PrincipalID, Role: authorization.RoleNamespaceAdmin,
	}})

	command := publication.CreateNamespace{Path: "acme/tools", OwnerPublisherID: owner}
	created, err := service.Create(context.Background(), actor, "request-1", command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Create(context.Background(), actor, "request-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID() != replayed.ID() || repository.creates != 1 || repository.auditPrincipal != actor.PrincipalID || transactions.calls != 2 {
		t.Fatalf("create/replay mismatch: %s %s, creates=%d transactions=%d", created.ID(), replayed.ID(), repository.creates, transactions.calls)
	}
	_, err = service.Create(context.Background(), actor, "request-1", publication.CreateNamespace{Path: "different", OwnerPublisherID: owner})
	assertTyped(t, err, shared.ErrorConflict, "idempotency.request_mismatch")
}

func TestNamespaceCreateRequiresExactAdministrativeGrant(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	repository := &namespaceMemory{verifiedOwners: map[shared.UUID]bool{owner: true}}
	service := newNamespaceService(t, repository, &idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "publisher-admin", Role: authorization.RolePublisherAdmin,
	}})
	_, err := service.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "publisher-admin"},
		"request-1", publication.CreateNamespace{Path: "acme", OwnerPublisherID: owner})
	assertTyped(t, err, shared.ErrorForbidden, "authorization.denied")
	if repository.creates != 0 {
		t.Fatal("unauthorized request reached persistence")
	}
}

func TestNamespaceCreateRejectsUnverifiedOwner(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	repository := &namespaceMemory{verifiedOwners: map[shared.UUID]bool{}}
	service := newNamespaceService(t, repository, &idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin,
	}})
	_, err := service.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "admin"},
		"request-1", publication.CreateNamespace{Path: "acme", OwnerPublisherID: owner})
	assertTyped(t, err, shared.ErrorConflict, "namespace.owner_not_verified")
}

func TestNamespaceCreateRejectsInvalidCommandBeforePersistence(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	repository := &namespaceMemory{}
	service := newNamespaceService(t, repository, &idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin,
	}})
	_, err := service.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "admin"},
		"request-1", publication.CreateNamespace{Path: "Invalid"})
	assertTyped(t, err, shared.ErrorInvalid, "namespace.invalid_request")
	if repository.creates != 0 {
		t.Fatal("invalid request reached persistence")
	}
}

func TestNamespaceReadAndOpaqueTenantBoundPagination(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	repository := &namespaceMemory{}
	for index, path := range []string{"alpha", "bravo/tools", "charlie"} {
		identifier := parseID(t, []string{
			"01991e80-2c00-7000-8000-000000000010",
			"01991e80-2c00-7000-8000-000000000011",
			"01991e80-2c00-7000-8000-000000000012",
		}[index])
		value, err := namespace.New(tenant, identifier, path, owner, fixedNow)
		if err != nil {
			t.Fatal(err)
		}
		repository.values = append(repository.values, value)
	}
	service := newNamespaceService(t, repository, &idempotencyMemory{}, &transactionMemory{}, nil)
	actor := identity.Identity{TenantID: tenant, PrincipalID: "reader"}

	got, err := service.Get(context.Background(), actor, repository.values[1].ID())
	if err != nil || got.Path() != "bravo/tools" {
		t.Fatalf("get: %q %v", got.Path(), err)
	}
	first, err := service.List(context.Background(), actor, 2, "")
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %#v %v", first, err)
	}
	second, err := service.List(context.Background(), actor, 2, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0].Path() != "charlie" || second.NextCursor != "" {
		t.Fatalf("second page: %#v %v", second, err)
	}
	other := identity.Identity{TenantID: parseID(t, "01991e80-2c00-7000-8000-000000000099"), PrincipalID: "reader"}
	_, err = service.List(context.Background(), other, 2, first.NextCursor)
	assertTyped(t, err, shared.ErrorInvalid, "namespace.invalid_cursor")
}

func newNamespaceService(t *testing.T, namespaces *namespaceMemory, records *idempotencyMemory, transactions *transactionMemory, grants []authorization.Grant) *publication.NamespaceService {
	t.Helper()
	authorizer, err := security.NewAuthorizer(grants)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := shared.NewUUIDGenerator(clock.Fixed{Time: fixedNow}, bytes.NewReader(bytes.Repeat([]byte{byte(len(namespaces.values) + 20)}, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	cursors, err := shared.NewCursorCodec(bytes.Repeat([]byte{8}, 32), clock.Fixed{Time: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	service, err := publication.NewNamespaceService(publication.NamespaceDependencies{
		Namespaces: namespaces, Idempotency: records, Transactions: transactions, Authorizer: authorizer,
		IDs: ids, Clock: clock.Fixed{Time: fixedNow}, Cursors: cursors,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type namespaceMemory struct {
	values         []namespace.Namespace
	verifiedOwners map[shared.UUID]bool
	creates        int
	auditPrincipal string
}

func (memory *namespaceMemory) Create(ctx context.Context, value namespace.Namespace) error {
	actor, ok := audit.ActorFromContext(ctx)
	if !ok {
		return errors.New("audit actor missing")
	}
	memory.auditPrincipal = actor.PrincipalID()
	if !memory.verifiedOwners[value.OwnerPublisherID()] {
		return typed(shared.ErrorConflict, "namespace.owner_not_verified")
	}
	for _, stored := range memory.values {
		if stored.TenantID() == value.TenantID() && stored.Path() == value.Path() {
			return typed(shared.ErrorConflict, "namespace.conflict")
		}
	}
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}

func (memory *namespaceMemory) Get(_ context.Context, tenantID, namespaceID shared.UUID) (namespace.Namespace, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == namespaceID {
			return value, nil
		}
	}
	return namespace.Namespace{}, typed(shared.ErrorNotFound, "namespace.not_found")
}

func (memory *namespaceMemory) GetByPath(_ context.Context, tenantID shared.UUID, path string) (namespace.Namespace, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.Path() == path {
			return value, nil
		}
	}
	return namespace.Namespace{}, typed(shared.ErrorNotFound, "namespace.not_found")
}

func (memory *namespaceMemory) List(_ context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]namespace.Namespace, error) {
	var result []namespace.Namespace
	for _, value := range memory.values {
		if value.TenantID() == tenantID && (after == nil || value.ID().String() > after.String()) {
			result = append(result, value)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}
