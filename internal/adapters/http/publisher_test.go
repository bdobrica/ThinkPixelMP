package httpadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

func TestPublisherHTTPCreateReadAndList(t *testing.T) {
	tenant := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000001")
	identifier := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000002")
	value, err := publisher.New(tenant, identifier, "acme", "Acme", "", time.Date(2026, 9, 6, 9, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	useCases := &publisherUseCaseFake{value: value}
	handler, err := NewPublisherHandler(useCases, authenticationFake{identity: identity.Identity{TenantID: tenant, PrincipalID: "admin"}})
	if err != nil {
		t.Fatal(err)
	}

	create := httptest.NewRequest(http.MethodPost, "/v1/publishers", strings.NewReader(`{"slug":"acme","display_name":"Acme"}`))
	create.Header.Set("Authorization", "Bearer credential")
	create.Header.Set(idempotencyKeyHeader, "create-1")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/v1/publishers/"+identifier.String() || created.Header().Get("ETag") != `"1"` {
		t.Fatalf("create response: %d %#v %s", created.Code, created.Header(), created.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || body["tenant_id"] != tenant.String() || body["created_at"] != "2026-09-06T09:30:00.000000000Z" {
		t.Fatalf("create body: %#v %v", body, err)
	}
	if useCases.key != "create-1" || useCases.command.Slug != "acme" || useCases.actor.TenantID != tenant || useCases.auditPrincipal != "admin" {
		t.Fatal("create inputs were not propagated")
	}

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/v1/publishers/"+identifier.String(), nil))
	if read.Code != http.StatusOK || useCases.gotID != identifier {
		t.Fatalf("read: %d %s", read.Code, read.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/publishers?page_size=25&cursor=opaque", nil))
	if list.Code != http.StatusOK || useCases.pageSize != 25 || useCases.cursor != "opaque" || !strings.Contains(list.Body.String(), `"items":[`) {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
}

func TestPublisherHTTPFailsClosedAtBoundary(t *testing.T) {
	tenant := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000001")
	identifier := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000002")
	value, _ := publisher.New(tenant, identifier, "acme", "", "", time.Now())
	useCases := &publisherUseCaseFake{value: value}
	authenticator := &authenticationFake{identity: identity.Identity{TenantID: tenant, PrincipalID: "admin"}, requireCredential: true}
	handler, err := NewPublisherHandler(useCases, authenticator)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, method, target, body, authorization, key string
		status                                         int
		code                                           string
	}{
		{"missing auth", http.MethodGet, "/v1/publishers", "", "", "", 401, "identity.missing"},
		{"wrong scheme", http.MethodGet, "/v1/publishers", "", "Basic abc", "", 401, "identity.invalid_authorization"},
		{"missing key", http.MethodPost, "/v1/publishers", `{"slug":"acme"}`, "Bearer credential", "", 400, "publisher.idempotency_key_required"},
		{"tenant injection", http.MethodPost, "/v1/publishers", `{"slug":"acme","tenant_id":"01991e80-2c00-7000-8000-000000000099"}`, "Bearer credential", "key", 400, "publisher.invalid_request"},
		{"invalid id", http.MethodGet, "/v1/publishers/not-a-uuid", "", "Bearer credential", "", 400, "publisher.invalid_id"},
		{"unknown query", http.MethodGet, "/v1/publishers?tenant_id=x", "", "Bearer credential", "", 400, "publisher.invalid_query"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			if test.key != "" {
				request.Header.Set(idempotencyKeyHeader, test.key)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
			if test.status == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("unauthorized response omitted Bearer challenge")
			}
		})
	}
}

type authenticationFake struct {
	identity          identity.Identity
	requireCredential bool
}

func (fake authenticationFake) Authenticate(_ context.Context, credential string) (identity.Identity, error) {
	if fake.requireCredential && credential == "" {
		return identity.Identity{}, httpTyped(shared.ErrorUnauthorized, "identity.missing")
	}
	return fake.identity, nil
}

type publisherUseCaseFake struct {
	value          publisher.Publisher
	actor          identity.Identity
	key            string
	command        publication.CreatePublisher
	gotID          shared.UUID
	pageSize       int
	cursor         string
	auditPrincipal string
}

func (fake *publisherUseCaseFake) Create(ctx context.Context, actor identity.Identity, key string, command publication.CreatePublisher) (publisher.Publisher, error) {
	fake.actor, fake.key, fake.command = actor, key, command
	if auditActor, ok := audit.ActorFromContext(ctx); ok {
		fake.auditPrincipal = auditActor.PrincipalID()
	}
	return fake.value, nil
}
func (fake *publisherUseCaseFake) Get(_ context.Context, _ identity.Identity, id shared.UUID) (publisher.Publisher, error) {
	fake.gotID = id
	return fake.value, nil
}
func (fake *publisherUseCaseFake) List(_ context.Context, _ identity.Identity, pageSize int, cursor string) (publication.PublisherPage, error) {
	fake.pageSize, fake.cursor = pageSize, cursor
	return publication.PublisherPage{Items: []publisher.Publisher{fake.value}, NextCursor: "next"}, nil
}

func mustHTTPID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
