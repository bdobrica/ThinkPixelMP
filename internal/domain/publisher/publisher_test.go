package publisher_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

func id(t *testing.T, value string) shared.UUID {
	t.Helper()
	parsed, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestPublisherValidationAndTransitions(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("offset", 3600))
	value, err := publisher.New(
		id(t, "0198fc21-ced5-7000-8000-000000000000"),
		id(t, "0198fc21-ced5-7000-8000-000000000001"),
		"acme-tools", "Acme Tools", "Enterprise publisher", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.State() != publisher.StateClaimed || value.StateVersion() != 1 || value.CreatedAt().Location() != time.UTC {
		t.Fatal("new publisher did not normalize initial state")
	}
	reason, _ := shared.NewReasonCode("ownership.confirmed")
	verified, record, err := value.Transition(publisher.StateVerified, reason, "Administrative verification", now.Add(time.Minute))
	if err != nil || verified.StateVersion() != 2 || record.State != publisher.StateVerified || record.Version != 2 {
		t.Fatalf("transition: %#v %#v %v", verified, record, err)
	}
	revoked, _, err := verified.Transition(publisher.StateRevoked, reason, "Trust withdrawn", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := revoked.Transition(publisher.StateVerified, reason, "invalid", now); !errors.Is(err, publisher.ErrInvalidTransition) {
		t.Fatal("revoked publisher accepted a transition")
	}
}

func TestPublisherRejectsInvalidValues(t *testing.T) {
	tenant := id(t, "0198fc21-ced5-7000-8000-000000000000")
	identifier := id(t, "0198fc21-ced5-7000-8000-000000000001")
	now := time.Now()
	for _, test := range []struct{ slug, display, description string }{
		{"Uppercase", "", ""}, {"-edge", "", ""}, {"edge-", "", ""},
		{strings.Repeat("a", 64), "", ""}, {"valid", "bad\nname", ""},
		{"valid", "", strings.Repeat("x", publisher.MaxDescriptionBytes+1)},
	} {
		if _, err := publisher.New(tenant, identifier, test.slug, test.display, test.description, now); err == nil {
			t.Fatalf("accepted invalid publisher: %#v", test)
		}
	}
}
