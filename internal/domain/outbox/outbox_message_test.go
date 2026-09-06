package outbox

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

func TestMessagePreservesStableEvent(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	id := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	payload := eventPayload(tenant, id, 7, now)
	message, err := New(tenant, id, 7, "urn:thinkpixel:mp:test", "io.thinkpixel.mp.artifact.registered.v1", id.String(), payload, now)
	if err != nil || message.State() != StatePending || message.Sequence() != 7 || string(message.Payload()) != string(payload) {
		t.Fatalf("new message: %#v %v", message, err)
	}
	copyPayload := message.Payload()
	copyPayload[0] = 'x'
	if string(message.Payload()) != string(payload) {
		t.Fatal("payload was mutable through accessor")
	}
}

func TestMessageRejectsMismatchedOrUnboundedValues(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	id := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	now := time.Now().UTC()
	payload := eventPayload(tenant, id, 1, now)
	for name, mutate := range map[string]func() (uint64, string, string, string, []byte){
		"sequence": func() (uint64, string, string, string, []byte) {
			return 2, "urn:test", "io.thinkpixel.mp.artifact.registered.v1", id.String(), payload
		},
		"type": func() (uint64, string, string, string, []byte) { return 1, "urn:test", "unsafe", id.String(), payload },
		"subject": func() (uint64, string, string, string, []byte) {
			return 1, "urn:test", "io.thinkpixel.mp.artifact.registered.v1", strings.Repeat("x", maxSubjectBytes+1), payload
		},
		"payload": func() (uint64, string, string, string, []byte) {
			return 1, "urn:test", "io.thinkpixel.mp.artifact.registered.v1", id.String(), []byte(`{}`)
		},
	} {
		sequence, source, eventType, subject, value := mutate()
		if _, err := New(tenant, id, sequence, source, eventType, subject, value, now); err == nil {
			t.Fatalf("accepted invalid %s", name)
		}
	}
	secretBearing := bytes.Replace(payload, []byte(`"transaction_cursor":"cursor-1"`), []byte(`"transaction_cursor":"cursor-1","authorization":"restricted"`), 1)
	if _, err := New(tenant, id, 1, "urn:thinkpixel:mp:test", "io.thinkpixel.mp.artifact.registered.v1", id.String(), secretBearing, now); err == nil {
		t.Fatal("accepted unknown event data")
	}
}

func TestMessageAcceptsBoundedNamespaceDelegationEvents(t *testing.T) {
	tenant := mustUUID(t, "0198fc21-ced5-7000-8000-000000000001")
	id := mustUUID(t, "0198fc21-ced5-7000-8000-000000000002")
	namespaceID := mustUUID(t, "0198fc21-ced5-7000-8000-000000000003")
	publisherID := mustUUID(t, "0198fc21-ced5-7000-8000-000000000004")
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for eventType, state := range map[string]string{
		"io.thinkpixel.mp.namespace.delegated.v1":          `"current_state":"active"`,
		"io.thinkpixel.mp.namespace.delegation-revoked.v1": `"previous_state":"active","current_state":"revoked","reason_code":"ownership.changed"`,
	} {
		payload := []byte(fmt.Sprintf(`{"specversion":"1.0","id":"%s","source":"urn:thinkpixel:mp:test","type":"%s","subject":"%s","time":"%s","datacontenttype":"%s","sequence":1,"data":{"tenant_id":"%s","transaction_cursor":"1","namespace_id":"%s","delegation_id":"%s","publisher_id":"%s",%s}}`,
			id.String(), eventType, id.String(), now.Format(time.RFC3339Nano), DataContentType, tenant.String(), namespaceID.String(), id.String(), publisherID.String(), state))
		if _, err := New(tenant, id, 1, "urn:thinkpixel:mp:test", eventType, id.String(), payload, now); err != nil {
			t.Fatalf("%s: %v", eventType, err)
		}
	}
}

func eventPayload(tenant, id shared.UUID, sequence uint64, at time.Time) []byte {
	return []byte(fmt.Sprintf(`{"specversion":"1.0","id":"%s","source":"urn:thinkpixel:mp:test","type":"io.thinkpixel.mp.artifact.registered.v1","subject":"%s","time":"%s","datacontenttype":"%s","sequence":%d,"data":{"tenant_id":"%s","transaction_cursor":"cursor-%d","artifact_version_id":"%s","artifact_digest":"sha256:%s","descriptor_digest":"sha256:%s"}}`,
		id.String(), id.String(), at.Format(time.RFC3339Nano), DataContentType, sequence, tenant.String(), sequence,
		id.String(), strings.Repeat("a", 64), strings.Repeat("b", 64)))
}

func mustUUID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
