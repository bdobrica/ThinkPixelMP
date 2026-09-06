package idempotency

import (
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

func TestRecordAndResultValidation(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	id := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	action, _ := shared.NewReasonCode("publisher.create")
	now := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	record, err := New(tenant, id, "oidc:subject", action, "request-1", shared.SHA256Digest([]byte("canonical")), now, now.Add(MinimumRetention))
	if err != nil || record.State() != StatePending || record.Principal() != "oidc:subject" {
		t.Fatalf("new record: %#v %v", record, err)
	}
	result, err := NewResult(201, "publisher", id.String())
	if err != nil || result.Status() != 201 {
		t.Fatalf("new result: %#v %v", result, err)
	}
	completed := now.Add(time.Second)
	restored, err := Restore(tenant, id, record.Principal(), action, record.Key(), record.RequestDigest(), StateCompleted, &result, now, &completed, record.ExpiresAt())
	if _, ok := restored.Result(); err != nil || !ok {
		t.Fatalf("restore completed: %#v %v", restored, err)
	}
}

func TestRejectsUnsafeOrInconsistentValues(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	id := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	action, _ := shared.NewReasonCode("publisher.create")
	now := time.Now().UTC()
	digest := shared.SHA256Digest(nil)
	for _, test := range []struct{ principal, key string }{
		{"", "key"}, {"principal", ""}, {"principal\nvalue", "key"}, {"principal", strings.Repeat("x", MaxKeyBytes+1)},
	} {
		if _, err := New(tenant, id, test.principal, action, test.key, digest, now, now.Add(MinimumRetention)); err == nil {
			t.Fatalf("accepted principal %q key %q", test.principal, test.key)
		}
	}
	if _, err := New(tenant, id, "principal", action, "key", digest, now, now.Add(MinimumRetention-time.Second)); err == nil {
		t.Fatal("accepted short retention")
	}
	if _, err := NewResult(99, "", ""); err == nil {
		t.Fatal("accepted invalid status")
	}
	if _, err := NewResult(200, "publisher", ""); err == nil {
		t.Fatal("accepted partial resource")
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
