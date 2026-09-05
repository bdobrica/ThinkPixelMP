package artifactversion_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
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

func digest(t *testing.T, character string) shared.Digest {
	t.Helper()
	value, err := shared.ParseDigest("sha256:" + strings.Repeat(character, 64))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestArtifactVersionPreservesCanonicalIdentity(t *testing.T) {
	semantic, _ := artifactversion.ParseSemanticVersion("1.2.3-rc.1+linux.amd64")
	now := time.Date(2026, 9, 5, 13, 0, 0, 0, time.FixedZone("offset", 3600))
	value, err := artifactversion.New(
		id(t, "0198fc21-ced5-7000-8000-000000000000"),
		id(t, "0198fc21-ced5-7000-8000-000000000001"),
		id(t, "0198fc21-ced5-7000-8000-000000000002"),
		id(t, "0198fc21-ced5-7000-8000-000000000003"),
		semantic, digest(t, "a"), artifact.KindSkill, artifactversion.ClassInstructional, artifactversion.DeliveryOCI, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.SemanticVersion().String() != "1.2.3-rc.1+linux.amd64" || value.Digest().String() != "sha256:"+strings.Repeat("a", 64) || value.Lifecycle() != artifactversion.LifecycleActive || value.RegisteredAt().Location() != time.UTC {
		t.Fatal("ArtifactVersion did not preserve canonical identity")
	}
}

func TestArtifactVersionRejectsInvalidContractValues(t *testing.T) {
	for _, value := range []string{"v1.2.3", "1.2", "01.2.3", "1.2.3-01", "1.2.3+", strings.Repeat("1", artifactversion.MaxSemanticVersionBytes+1)} {
		if _, err := artifactversion.ParseSemanticVersion(value); err == nil {
			t.Fatalf("accepted invalid semantic version %q", value)
		}
	}
	tenant := id(t, "0198fc21-ced5-7000-8000-000000000000")
	identifier := id(t, "0198fc21-ced5-7000-8000-000000000001")
	artifactID := id(t, "0198fc21-ced5-7000-8000-000000000002")
	publisherID := id(t, "0198fc21-ced5-7000-8000-000000000003")
	semantic, _ := artifactversion.ParseSemanticVersion("1.2.3")
	validDigest := digest(t, "b")
	now := time.Now()
	for name, build := range map[string]func() error{
		"invalid kind/class": func() error {
			_, err := artifactversion.New(tenant, identifier, artifactID, publisherID, semantic, validDigest, artifact.KindBundle, artifactversion.ClassInstructional, artifactversion.DeliveryOCI, now)
			return err
		},
		"invalid delivery": func() error {
			_, err := artifactversion.New(tenant, identifier, artifactID, publisherID, semantic, validDigest, artifact.KindAgentRuntime, artifactversion.ClassExecutableLocal, artifactversion.DeliveryRemote, now)
			return err
		},
		"invalid lifecycle": func() error {
			_, err := artifactversion.Restore(tenant, identifier, artifactID, publisherID, semantic, validDigest, artifact.KindSkill, artifactversion.ClassInstructional, artifactversion.DeliveryOCI, "unknown", now)
			return err
		},
		"zero time": func() error {
			_, err := artifactversion.New(tenant, identifier, artifactID, publisherID, semantic, validDigest, artifact.KindSkill, artifactversion.ClassInstructional, artifactversion.DeliveryOCI, time.Time{})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if build() == nil {
				t.Fatal("accepted invalid ArtifactVersion")
			}
		})
	}
}

func TestArtifactVersionAcceptsTaxonomyCombinations(t *testing.T) {
	semantic, _ := artifactversion.ParseSemanticVersion("1.0.0")
	identifiers := []shared.UUID{
		id(t, "0198fc21-ced5-7000-8000-000000000000"), id(t, "0198fc21-ced5-7000-8000-000000000001"),
		id(t, "0198fc21-ced5-7000-8000-000000000002"), id(t, "0198fc21-ced5-7000-8000-000000000003"),
	}
	for _, test := range []struct {
		kind     artifact.Kind
		class    artifactversion.Class
		delivery artifactversion.DeliveryModel
	}{
		{artifact.KindSkill, artifactversion.ClassExecutableLocal, artifactversion.DeliveryOCI},
		{artifact.KindMCPServer, artifactversion.ClassRemoteService, artifactversion.DeliveryRemote},
		{artifact.KindRemoteAgent, artifactversion.ClassRemoteService, artifactversion.DeliveryOCI},
		{artifact.KindBundle, artifactversion.ClassComposite, artifactversion.DeliveryImportedSource},
	} {
		if _, err := artifactversion.New(identifiers[0], identifiers[1], identifiers[2], identifiers[3], semantic, digest(t, "c"), test.kind, test.class, test.delivery, time.Now()); err != nil {
			t.Fatalf("rejected valid taxonomy combination: %v", err)
		}
	}
}
