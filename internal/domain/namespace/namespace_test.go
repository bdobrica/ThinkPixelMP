package namespace_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
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

func TestNamespaceValidation(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("offset", 3600))
	value, err := namespace.New(
		id(t, "0198fc21-ced5-7000-8000-000000000000"),
		id(t, "0198fc21-ced5-7000-8000-000000000001"),
		"acme/security-tools",
		id(t, "0198fc21-ced5-7000-8000-000000000002"),
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.Path() != "acme/security-tools" || value.CreatedAt().Location() != time.UTC {
		t.Fatal("new namespace did not preserve its canonical path and normalize time")
	}
}

func TestNamespaceRejectsInvalidPaths(t *testing.T) {
	tenant := id(t, "0198fc21-ced5-7000-8000-000000000000")
	identifier := id(t, "0198fc21-ced5-7000-8000-000000000001")
	owner := id(t, "0198fc21-ced5-7000-8000-000000000002")
	for _, path := range []string{"", "Uppercase", "/acme", "acme/", "acme//tools", "acme/.", "acme/..", "acme/_tools", strings.Repeat("a", namespace.MaxPathBytes+1)} {
		if _, err := namespace.New(tenant, identifier, path, owner, time.Now()); err == nil {
			t.Fatalf("accepted invalid path %q", path)
		}
	}
}

func TestDelegationRequiresStrictChildAndHasAppendOnlyRevocation(t *testing.T) {
	tenant := id(t, "0198fc21-ced5-7000-8000-000000000000")
	identifier := id(t, "0198fc21-ced5-7000-8000-000000000001")
	root := id(t, "0198fc21-ced5-7000-8000-000000000002")
	publisher := id(t, "0198fc21-ced5-7000-8000-000000000003")
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for _, prefix := range []string{"acme", "acme-other/tools", "other/tools"} {
		if _, err := namespace.NewDelegation(tenant, identifier, root, "acme", publisher, prefix, now); err == nil {
			t.Fatalf("accepted non-child prefix %q", prefix)
		}
	}
	delegation, err := namespace.NewDelegation(tenant, identifier, root, "acme", publisher, "acme/security/tools", now)
	if err != nil {
		t.Fatal(err)
	}
	reason, _ := shared.NewReasonCode("ownership.changed")
	revoked, err := delegation.Revoke(reason, "reassigned", now.Add(time.Minute))
	if err != nil || revoked.State() != namespace.DelegationRevoked || revoked.StateVersion() != 2 {
		t.Fatalf("revocation: %#v %v", revoked, err)
	}
	if _, err := revoked.Revoke(reason, "again", now.Add(2*time.Minute)); err == nil {
		t.Fatal("revoked a terminal delegation")
	}
}
