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
