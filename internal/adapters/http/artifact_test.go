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
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

func TestArtifactHTTPCreateReadAndList(t *testing.T) {
	tenant := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000001")
	namespaceID := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000002")
	identifier := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000003")
	value, err := artifact.New(tenant, identifier, namespaceID, "acme/tools", "reviewer", artifact.KindSkill,
		"Reviewer", "Checks changes", "https://example.test/reviewer", "https://example.test/source",
		map[string]string{"security/tier": "reviewed"}, time.Date(2026, 9, 6, 9, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	useCases := &artifactUseCaseFake{value: value}
	handler, err := NewArtifactHandler(useCases, authenticationFake{identity: identity.Identity{TenantID: tenant, PrincipalID: "publisher"}})
	if err != nil {
		t.Fatal(err)
	}

	create := httptest.NewRequest(http.MethodPost, "/v1/artifacts", strings.NewReader(`{"namespace_id":"`+namespaceID.String()+`","name":"reviewer","kind":"skill","labels":{"security/tier":"reviewed"}}`))
	create.Header.Set("Authorization", "Bearer credential")
	create.Header.Set(idempotencyKeyHeader, "create-1")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/v1/artifacts/"+identifier.String() || created.Header().Get("ETag") != "" {
		t.Fatalf("create response: %d %#v %s", created.Code, created.Header(), created.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || body["tenant_id"] != tenant.String() ||
		body["identity"] != "acme/tools/reviewer" || body["created_at"] != "2026-09-06T09:30:00.000000000Z" {
		t.Fatalf("create body: %#v %v", body, err)
	}
	if useCases.key != "create-1" || useCases.command.NamespaceID != namespaceID || useCases.command.Kind != artifact.KindSkill ||
		useCases.actor.TenantID != tenant || useCases.auditPrincipal != "publisher" {
		t.Fatal("create inputs were not propagated")
	}

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/v1/artifacts/"+identifier.String(), nil))
	if read.Code != http.StatusOK || useCases.gotID != identifier {
		t.Fatalf("read: %d %s", read.Code, read.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/artifacts?page_size=25&query=review&cursor=opaque", nil))
	if list.Code != http.StatusOK || useCases.pageSize != 25 || useCases.query != "review" || useCases.cursor != "opaque" || !strings.Contains(list.Body.String(), `"items":[`) {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
}

func TestArtifactHTTPFailsClosedAtBoundary(t *testing.T) {
	tenant := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000001")
	namespaceID := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000002")
	identifier := mustHTTPID(t, "01991e80-2c00-7000-8000-000000000003")
	value, _ := artifact.New(tenant, identifier, namespaceID, "acme", "reviewer", artifact.KindSkill, "", "", "", "", nil, time.Now())
	useCases := &artifactUseCaseFake{value: value}
	handler, err := NewArtifactHandler(useCases, &authenticationFake{identity: identity.Identity{TenantID: tenant, PrincipalID: "publisher"}, requireCredential: true})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, method, target, body, authorization, key string
		status                                         int
		code                                           string
	}{
		{"missing auth", http.MethodGet, "/v1/artifacts", "", "", "", 401, "identity.missing"},
		{"missing key", http.MethodPost, "/v1/artifacts", `{"namespace_id":"` + namespaceID.String() + `","name":"reviewer","kind":"skill"}`, "Bearer credential", "", 400, "artifact.idempotency_key_required"},
		{"tenant injection", http.MethodPost, "/v1/artifacts", `{"namespace_id":"` + namespaceID.String() + `","name":"reviewer","kind":"skill","tenant_id":"` + tenant.String() + `"}`, "Bearer credential", "key", 400, "artifact.invalid_request"},
		{"invalid namespace", http.MethodPost, "/v1/artifacts", `{"namespace_id":"nope","name":"reviewer","kind":"skill"}`, "Bearer credential", "key", 400, "artifact.invalid_request"},
		{"invalid id", http.MethodGet, "/v1/artifacts/not-a-uuid", "", "Bearer credential", "", 400, "artifact.invalid_id"},
		{"unknown query", http.MethodGet, "/v1/artifacts?tenant_id=x", "", "Bearer credential", "", 400, "artifact.invalid_query"},
		{"duplicate query", http.MethodGet, "/v1/artifacts?query=a&query=b", "", "Bearer credential", "", 400, "artifact.invalid_query"},
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
		})
	}
}

type artifactUseCaseFake struct {
	value          artifact.Artifact
	actor          identity.Identity
	key            string
	command        publication.CreateArtifact
	gotID          shared.UUID
	query          string
	pageSize       int
	cursor         string
	auditPrincipal string
}

func (fake *artifactUseCaseFake) Create(ctx context.Context, actor identity.Identity, key string, command publication.CreateArtifact) (artifact.Artifact, error) {
	fake.actor, fake.key, fake.command = actor, key, command
	if auditActor, ok := audit.ActorFromContext(ctx); ok {
		fake.auditPrincipal = auditActor.PrincipalID()
	}
	return fake.value, nil
}

func (fake *artifactUseCaseFake) Get(_ context.Context, _ identity.Identity, id shared.UUID) (artifact.Artifact, error) {
	fake.gotID = id
	return fake.value, nil
}

func (fake *artifactUseCaseFake) List(_ context.Context, _ identity.Identity, query string, pageSize int, cursor string) (publication.ArtifactPage, error) {
	fake.query, fake.pageSize, fake.cursor = query, pageSize, cursor
	return publication.ArtifactPage{Items: []artifact.Artifact{fake.value}, NextCursor: "next"}, nil
}
