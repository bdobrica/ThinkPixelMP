package shared_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
)

func TestUUIDv7UsesInjectedClockAndCanonicalParsing(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 34, 56, 789000000, time.FixedZone("test", 7200))
	generator, err := shared.NewUUIDGenerator(clock.Fixed{Time: now}, bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := generator.New()
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != "01a05762-ef95-7000-8000-000000000000" {
		t.Fatalf("unexpected UUID %s", id)
	}
	if parsed, err := shared.ParseUUID(id.String()); err != nil || parsed != id {
		t.Fatalf("parse UUID: %v", err)
	}
	for _, invalid := range []string{"0198FC21-ced5-7000-8000-000000000000", "0198fc21-ced5-4000-8000-000000000000", ""} {
		if _, err := shared.ParseUUID(invalid); err == nil {
			t.Errorf("accepted invalid UUID %q", invalid)
		}
	}
}

func TestDigestAndArtifactReferenceAreCanonical(t *testing.T) {
	digestText := "sha256:" + strings.Repeat("a", 64)
	digest, err := shared.ParseDigest(digestText)
	if err != nil {
		t.Fatal(err)
	}
	if digest.String() != digestText || digest.Algorithm() != shared.SHA256 {
		t.Fatalf("unexpected digest %q", digest)
	}
	if shared.SHA256Digest(nil).String() != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal("unexpected SHA-256 computation")
	}
	for _, invalid := range []string{"SHA256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("A", 64), "sha256:abc"} {
		if _, err := shared.ParseDigest(invalid); err == nil {
			t.Errorf("accepted invalid digest %q", invalid)
		}
	}
	reference, err := shared.ParseArtifactReference("enterprise/security/reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if reference.Namespace() != "enterprise/security" || reference.Name() != "reviewer" || reference.String() != "enterprise/security/reviewer" {
		t.Fatalf("unexpected reference %#v", reference)
	}
	for _, invalid := range []string{"reviewer", "Enterprise/reviewer", "enterprise//reviewer", "enterprise/-reviewer", strings.Repeat("a/", 129) + "name"} {
		if _, err := shared.ParseArtifactReference(invalid); err == nil {
			t.Errorf("accepted invalid reference %q", invalid)
		}
	}
}

func TestBoundedValuesAndTypedErrors(t *testing.T) {
	value, err := shared.NewBoundedString("safe explanation", 32)
	if err != nil || value.String() != "safe explanation" {
		t.Fatalf("bounded string: %v", err)
	}
	for _, input := range []string{"", "line\nbreak", strings.Repeat("x", 33)} {
		if _, err := shared.NewBoundedString(input, 32); err == nil {
			t.Errorf("accepted invalid bounded string %q", input)
		}
	}
	code, err := shared.NewReasonCode("artifact.digest_mismatch")
	if err != nil {
		t.Fatal(err)
	}
	problem := shared.NewTypedError(shared.ErrorConflict, code)
	var typed *shared.TypedError
	if !errors.As(problem, &typed) || typed.Class() != shared.ErrorConflict || typed.Code() != code {
		t.Fatalf("unexpected typed error %v", problem)
	}
	if strings.Contains(problem.Error(), "secret") {
		t.Fatal("typed error leaked detail")
	}
}

func TestCursorAuthenticatesAndBindsEveryScopeField(t *testing.T) {
	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	tenant, _ := shared.ParseUUID("0198fc21-ced5-7000-8000-000000000000")
	query, _ := shared.ParseDigest("sha256:" + strings.Repeat("b", 64))
	scope := shared.CursorScope{TenantID: tenant, Endpoint: "/v1/artifacts", QueryDigest: query, PageSize: 50}
	codec, err := shared.NewCursorCodec(bytes.Repeat([]byte{0x42}, 32), clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := codec.Encode(scope, shared.Cursor{Position: "sort-key|id", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Decode(encoded, scope)
	if err != nil || decoded.Position != "sort-key|id" {
		t.Fatalf("decode: %#v %v", decoded, err)
	}

	tampered := encoded[:len(encoded)-1] + "A"
	if _, err := codec.Decode(tampered, scope); err == nil {
		t.Fatal("accepted tampered cursor")
	}
	otherScope := scope
	otherScope.Endpoint = "/v1/catalogs"
	if _, err := codec.Decode(encoded, otherScope); err == nil {
		t.Fatal("accepted cross-endpoint cursor")
	}
	otherScope = scope
	otherScope.PageSize = 51
	if _, err := codec.Decode(encoded, otherScope); err == nil {
		t.Fatal("accepted different page size")
	}
	otherScope = scope
	otherScope.QueryDigest = shared.SHA256Digest([]byte("different filters"))
	if _, err := codec.Decode(encoded, otherScope); err == nil {
		t.Fatal("accepted different query shape")
	}
	otherScope = scope
	otherScope.TenantID, _ = shared.ParseUUID("0198fc21-ced5-7000-8000-000000000001")
	if _, err := codec.Decode(encoded, otherScope); err == nil {
		t.Fatal("accepted cross-tenant cursor")
	}
	expiredCodec, _ := shared.NewCursorCodec(bytes.Repeat([]byte{0x42}, 32), clock.Fixed{Time: now.Add(2 * time.Hour)})
	if _, err := expiredCodec.Decode(encoded, scope); err == nil {
		t.Fatal("accepted expired cursor")
	}
}

func TestCursorRejectsWeakKeysAndUnsafeScopes(t *testing.T) {
	now := clock.Fixed{Time: time.Now()}
	if _, err := shared.NewCursorCodec([]byte("short"), now); err == nil {
		t.Fatal("accepted weak key")
	}
	codec, _ := shared.NewCursorCodec(bytes.Repeat([]byte{1}, 32), now)
	if _, err := codec.Encode(shared.CursorScope{}, shared.Cursor{}); err == nil {
		t.Fatal("accepted empty scope")
	}
}
