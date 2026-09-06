package audit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

func TestActorAndEventValidation(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	eventID := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	requestID := mustUUID(t, "0198fc21-ced5-7000-8000-000000000003")
	action, _ := shared.NewReasonCode("artifact.registered")
	resourceType, _ := shared.NewReasonCode("artifact_version")
	actor, err := NewActor("oidc:issuer:subject", &requestID, strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithActor(context.Background(), actor)
	if got, ok := ActorFromContext(ctx); !ok || got.PrincipalID() != "oidc:issuer:subject" {
		t.Fatal("actor context did not round trip")
	}
	event, err := Restore(tenant, eventID, actor.PrincipalID(), action, resourceType, eventID.String(), nil, nil, nil, nil, nil, time.Now(), &requestID, actor.TraceID())
	if err != nil || event.TenantID() != tenant || event.ID() != eventID {
		t.Fatalf("restore: %#v %v", event, err)
	}
}

func TestRejectsUnsafeAuditValues(t *testing.T) {
	if _, err := NewActor("token\nvalue", nil, ""); err == nil {
		t.Fatal("accepted control character")
	}
	if _, err := NewActor("actor", nil, "not-a-trace"); err == nil {
		t.Fatal("accepted invalid trace ID")
	}
	if _, ok := ActorFromContext(context.Background()); ok {
		t.Fatal("found absent actor")
	}
}

func mustUUID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
