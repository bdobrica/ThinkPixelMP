package publication_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/security"
)

func TestArtifactCreateIsAuthorizedOwnedAtomicAndReplaySafe(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	namespaceID := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	root, _ := namespace.New(tenant, namespaceID, "acme/security", owner, fixedNow)
	repository := &artifactMemory{}
	records := &idempotencyMemory{}
	transactions := &transactionMemory{}
	service := newArtifactService(t, repository, &namespaceMemory{values: []namespace.Namespace{root}}, ownerResolver{owner: owner}, records, transactions, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin,
	}})
	actor := identity.Identity{TenantID: tenant, PrincipalID: "publisher"}
	command := publication.CreateArtifact{NamespaceID: namespaceID, Name: "reviewer", Kind: artifact.KindSkill,
		DisplayName: "Reviewer", Labels: map[string]string{"security/tier": "reviewed"}}

	created, err := service.Create(context.Background(), actor, "request-1", command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Create(context.Background(), actor, "request-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID() != replayed.ID() || created.Identity().String() != "acme/security/reviewer" || repository.creates != 1 ||
		repository.auditPrincipal != actor.PrincipalID || transactions.calls != 2 {
		t.Fatalf("create/replay mismatch: %#v %#v creates=%d transactions=%d", created, replayed, repository.creates, transactions.calls)
	}
	_, err = service.Create(context.Background(), actor, "request-1", publication.CreateArtifact{NamespaceID: namespaceID, Name: "different", Kind: artifact.KindSkill})
	assertTyped(t, err, shared.ErrorConflict, "idempotency.request_mismatch")
}

func TestArtifactCreateRequiresExactRoleAndLiveRootOwner(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	other := parseID(t, "01991e80-2c00-7000-8000-000000000004")
	namespaceID := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	root, _ := namespace.New(tenant, namespaceID, "acme", owner, fixedNow)
	command := publication.CreateArtifact{NamespaceID: namespaceID, Name: "reviewer", Kind: artifact.KindSkill}

	unauthorizedRepository := &artifactMemory{}
	unauthorized := newArtifactService(t, unauthorizedRepository, &namespaceMemory{values: []namespace.Namespace{root}}, ownerResolver{owner: owner},
		&idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin}})
	_, err := unauthorized.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "admin"}, "request-1", command)
	assertTyped(t, err, shared.ErrorForbidden, "authorization.denied")
	if unauthorizedRepository.creates != 0 {
		t.Fatal("unauthorized create reached persistence")
	}

	inactiveRepository := &artifactMemory{}
	inactive := newArtifactService(t, inactiveRepository, &namespaceMemory{values: []namespace.Namespace{root}}, ownerResolver{owner: other},
		&idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin}})
	_, err = inactive.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "publisher"}, "request-1", command)
	assertTyped(t, err, shared.ErrorForbidden, "namespace.publication_not_authorized")
	if inactiveRepository.creates != 0 {
		t.Fatal("ownership failure reached artifact persistence")
	}
}

func TestArtifactReadAndQueryBoundPagination(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	namespaceID := parseID(t, "01991e80-2c00-7000-8000-000000000003")
	repository := &artifactMemory{}
	for index, name := range []string{"alpha", "bravo", "charlie"} {
		identifier := parseID(t, []string{"01991e80-2c00-7000-8000-000000000010", "01991e80-2c00-7000-8000-000000000011", "01991e80-2c00-7000-8000-000000000012"}[index])
		value, err := artifact.New(tenant, identifier, namespaceID, "acme", name, artifact.KindSkill, strings.ToUpper(name), "", "", "", nil, fixedNow)
		if err != nil {
			t.Fatal(err)
		}
		repository.values = append(repository.values, value)
	}
	service := newArtifactService(t, repository, &namespaceMemory{}, ownerResolver{}, &idempotencyMemory{}, &transactionMemory{}, nil)
	actor := identity.Identity{TenantID: tenant, PrincipalID: "reader"}

	got, err := service.Get(context.Background(), actor, repository.values[1].ID())
	if err != nil || got.Name() != "bravo" {
		t.Fatalf("get: %#v %v", got, err)
	}
	first, err := service.List(context.Background(), actor, "a", 2, "")
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %#v %v", first, err)
	}
	second, err := service.List(context.Background(), actor, "a", 2, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0].Name() != "charlie" {
		t.Fatalf("second page: %#v %v", second, err)
	}
	_, err = service.List(context.Background(), actor, "different", 2, first.NextCursor)
	assertTyped(t, err, shared.ErrorInvalid, "artifact.invalid_cursor")
	otherTenant := identity.Identity{TenantID: parseID(t, "01991e80-2c00-7000-8000-000000000099"), PrincipalID: "reader"}
	_, err = service.List(context.Background(), otherTenant, "a", 2, first.NextCursor)
	assertTyped(t, err, shared.ErrorInvalid, "artifact.invalid_cursor")
}

func newArtifactService(t *testing.T, artifacts *artifactMemory, namespaces *namespaceMemory, owners ownerResolver,
	records *idempotencyMemory, transactions *transactionMemory, grants []authorization.Grant,
) *publication.ArtifactService {
	t.Helper()
	authorizer, err := security.NewAuthorizer(grants)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := shared.NewUUIDGenerator(clock.Fixed{Time: fixedNow}, bytes.NewReader(bytes.Repeat([]byte{31}, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	cursors, err := shared.NewCursorCodec(bytes.Repeat([]byte{9}, 32), clock.Fixed{Time: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	service, err := publication.NewArtifactService(publication.ArtifactDependencies{Artifacts: artifacts, Namespaces: namespaces,
		Owners: owners, Idempotency: records, Transactions: transactions, Authorizer: authorizer, IDs: ids,
		Clock: clock.Fixed{Time: fixedNow}, Cursors: cursors})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type ownerResolver struct {
	owner shared.UUID
	err   error
}

func (resolver ownerResolver) ResolveOwner(context.Context, shared.UUID, string) (shared.UUID, error) {
	return resolver.owner, resolver.err
}

type artifactMemory struct {
	values         []artifact.Artifact
	creates        int
	auditPrincipal string
}

func (memory *artifactMemory) Create(ctx context.Context, value artifact.Artifact) error {
	actor, ok := audit.ActorFromContext(ctx)
	if !ok {
		return errors.New("audit actor missing")
	}
	memory.auditPrincipal = actor.PrincipalID()
	for _, stored := range memory.values {
		if stored.TenantID() == value.TenantID() && stored.Identity() == value.Identity() {
			return typed(shared.ErrorConflict, "artifact.conflict")
		}
	}
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}

func (memory *artifactMemory) Get(_ context.Context, tenantID, artifactID shared.UUID) (artifact.Artifact, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == artifactID {
			return value, nil
		}
	}
	return artifact.Artifact{}, typed(shared.ErrorNotFound, "artifact.not_found")
}

func (memory *artifactMemory) GetByIdentity(_ context.Context, tenantID shared.UUID, identity shared.ArtifactReference) (artifact.Artifact, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.Identity() == identity {
			return value, nil
		}
	}
	return artifact.Artifact{}, typed(shared.ErrorNotFound, "artifact.not_found")
}

func (memory *artifactMemory) List(_ context.Context, tenantID shared.UUID, query string, after *shared.UUID, limit int) ([]artifact.Artifact, error) {
	query = strings.ToLower(query)
	var result []artifact.Artifact
	for _, value := range memory.values {
		matches := query == "" || strings.Contains(strings.ToLower(value.Identity().String()), query) || strings.Contains(strings.ToLower(value.DisplayName()), query)
		if value.TenantID() == tenantID && matches && (after == nil || value.ID().String() > after.String()) {
			result = append(result, value)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}
