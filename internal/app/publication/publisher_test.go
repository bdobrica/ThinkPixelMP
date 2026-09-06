package publication_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/security"
)

var fixedNow = time.Date(2026, 9, 6, 9, 30, 0, 123000000, time.UTC)

func TestPublisherCreateIsAuthorizedAtomicAndReplaySafe(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	actor := identity.Identity{TenantID: tenant, PrincipalID: "principal-1"}
	repository := &publisherMemory{}
	idempotencyRepository := &idempotencyMemory{}
	transactions := &transactionMemory{}
	service := newService(t, repository, idempotencyRepository, transactions, []authorization.Grant{{
		TenantID: tenant, PrincipalID: actor.PrincipalID, Role: authorization.RolePublisherAdmin,
	}})

	command := publication.CreatePublisher{Slug: "acme-tools", DisplayName: "Acme Tools", Description: "Enterprise tools"}
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
	_, err = service.Create(context.Background(), actor, "request-1", publication.CreatePublisher{Slug: "different"})
	assertTyped(t, err, shared.ErrorConflict, "idempotency.request_mismatch")
}

func TestPublisherCreateRequiresExactAdministrativeGrant(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	repository := &publisherMemory{}
	service := newService(t, repository, &idempotencyMemory{}, &transactionMemory{}, []authorization.Grant{{
		TenantID: tenant, PrincipalID: "reader", Role: authorization.RoleNamespaceAdmin,
	}})
	_, err := service.Create(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "reader"}, "request-1", publication.CreatePublisher{Slug: "acme"})
	assertTyped(t, err, shared.ErrorForbidden, "authorization.denied")
	if repository.creates != 0 {
		t.Fatal("unauthorized request reached persistence")
	}
}

func TestPublisherReadAndOpaqueTenantBoundPagination(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	repository := &publisherMemory{}
	for index, slug := range []string{"alpha", "bravo", "charlie"} {
		identifier := parseID(t, []string{
			"01991e80-2c00-7000-8000-000000000010",
			"01991e80-2c00-7000-8000-000000000011",
			"01991e80-2c00-7000-8000-000000000012",
		}[index])
		value, err := publisher.New(tenant, identifier, slug, "", "", fixedNow)
		if err != nil {
			t.Fatal(err)
		}
		repository.values = append(repository.values, value)
	}
	service := newService(t, repository, &idempotencyMemory{}, &transactionMemory{}, nil)
	actor := identity.Identity{TenantID: tenant, PrincipalID: "reader"}

	got, err := service.Get(context.Background(), actor, repository.values[1].ID())
	if err != nil || got.Slug() != "bravo" {
		t.Fatalf("get: %q %v", got.Slug(), err)
	}
	first, err := service.List(context.Background(), actor, 2, "")
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %#v %v", first, err)
	}
	second, err := service.List(context.Background(), actor, 2, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0].Slug() != "charlie" || second.NextCursor != "" {
		t.Fatalf("second page: %#v %v", second, err)
	}
	other := identity.Identity{TenantID: parseID(t, "01991e80-2c00-7000-8000-000000000099"), PrincipalID: "reader"}
	_, err = service.List(context.Background(), other, 2, first.NextCursor)
	assertTyped(t, err, shared.ErrorInvalid, "publisher.invalid_cursor")
}

func newService(t *testing.T, publishers *publisherMemory, records *idempotencyMemory, transactions *transactionMemory, grants []authorization.Grant) *publication.PublisherService {
	t.Helper()
	authorizer, err := security.NewAuthorizer(grants)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := shared.NewUUIDGenerator(clock.Fixed{Time: fixedNow}, bytes.NewReader(bytes.Repeat([]byte{byte(len(publishers.values) + 1)}, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	cursors, err := shared.NewCursorCodec(bytes.Repeat([]byte{7}, 32), clock.Fixed{Time: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	service, err := publication.NewPublisherService(publication.PublisherDependencies{
		Publishers: publishers, Idempotency: records, Transactions: transactions, Authorizer: authorizer,
		IDs: ids, Clock: clock.Fixed{Time: fixedNow}, Cursors: cursors,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type publisherMemory struct {
	values         []publisher.Publisher
	creates        int
	auditPrincipal string
}

func (memory *publisherMemory) Create(ctx context.Context, value publisher.Publisher) error {
	actor, ok := audit.ActorFromContext(ctx)
	if !ok {
		return errors.New("audit actor missing")
	}
	memory.auditPrincipal = actor.PrincipalID()
	for _, stored := range memory.values {
		if stored.TenantID() == value.TenantID() && stored.Slug() == value.Slug() {
			return typed(shared.ErrorConflict, "publisher.conflict")
		}
	}
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}
func (memory *publisherMemory) Get(_ context.Context, tenantID, publisherID shared.UUID) (publisher.Publisher, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == publisherID {
			return value, nil
		}
	}
	return publisher.Publisher{}, typed(shared.ErrorNotFound, "publisher.not_found")
}
func (memory *publisherMemory) GetBySlug(_ context.Context, tenantID shared.UUID, slug string) (publisher.Publisher, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.Slug() == slug {
			return value, nil
		}
	}
	return publisher.Publisher{}, typed(shared.ErrorNotFound, "publisher.not_found")
}
func (memory *publisherMemory) List(_ context.Context, tenantID shared.UUID, after *shared.UUID, limit int) ([]publisher.Publisher, error) {
	var result []publisher.Publisher
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
func (*publisherMemory) ChangeState(context.Context, shared.UUID, shared.UUID, int64, publisher.State, shared.ReasonCode, string, time.Time) (publisher.Publisher, error) {
	return publisher.Publisher{}, errors.New("not implemented")
}

type idempotencyMemory struct{ record *idempotency.Record }

func (memory *idempotencyMemory) Acquire(_ context.Context, value idempotency.Record) (idempotency.Record, bool, error) {
	if memory.record == nil {
		copyValue := value
		memory.record = &copyValue
		return value, true, nil
	}
	if memory.record.RequestDigest() != value.RequestDigest() {
		return idempotency.Record{}, false, typed(shared.ErrorConflict, "idempotency.request_mismatch")
	}
	return *memory.record, false, nil
}
func (memory *idempotencyMemory) Get(context.Context, shared.UUID, string, shared.ReasonCode, string) (idempotency.Record, error) {
	if memory.record == nil {
		return idempotency.Record{}, errors.New("missing")
	}
	return *memory.record, nil
}
func (memory *idempotencyMemory) Complete(_ context.Context, tenantID shared.UUID, principal string, action shared.ReasonCode, key string, digest shared.Digest, result idempotency.Result, at time.Time) (idempotency.Record, error) {
	completed, err := idempotency.Restore(tenantID, memory.record.ID(), principal, action, key, digest, idempotency.StateCompleted, &result, memory.record.CreatedAt(), &at, memory.record.ExpiresAt())
	if err == nil {
		memory.record = &completed
	}
	return completed, err
}

type transactionMemory struct{ calls int }

func (memory *transactionMemory) WithinTransaction(ctx context.Context, _ shared.UUID, operation func(context.Context) error) error {
	memory.calls++
	return operation(ctx)
}

func parseID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func typed(class shared.ErrorClass, value string) error {
	code, _ := shared.NewReasonCode(value)
	return shared.NewTypedError(class, code)
}
func assertTyped(t *testing.T, err error, class shared.ErrorClass, code string) {
	t.Helper()
	var value *shared.TypedError
	if !errors.As(err, &value) || value.Class() != class || value.Code().String() != code {
		t.Fatalf("unexpected error: %v", err)
	}
}
