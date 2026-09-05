package artifactdescriptor_test

import (
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdescriptor"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

func mustID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestArtifactDescriptorPreservesBoundedNormalizedMetadata(t *testing.T) {
	metadata := []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme/security","name":"reviewer","version":"1.2.3"},"requirements":{},"dependencies":[],"spec":{}}`)
	descriptor, err := artifactdescriptor.New(
		mustID(t, "0198fc21-ced5-7000-8000-000000000000"),
		mustID(t, "0198fc21-ced5-7000-8000-000000000001"),
		shared.SHA256Digest(metadata),
		"application/vnd.thinkpixel.skill.manifest.v1+json",
		metadata,
	)
	if err != nil || descriptor.Kind() != artifact.KindSkill || descriptor.Identity().String() != "acme/security/reviewer" || descriptor.SemanticVersion().String() != "1.2.3" {
		t.Fatalf("descriptor: %#v %v", descriptor, err)
	}
	returned := descriptor.NormalizedMetadata()
	returned[0] = '['
	if descriptor.NormalizedMetadata()[0] != '{' {
		t.Fatal("normalized metadata was mutable through accessor")
	}
}

func TestArtifactDescriptorRejectsInvalidMetadata(t *testing.T) {
	tenantID := mustID(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := mustID(t, "0198fc21-ced5-7000-8000-000000000001")
	valid := []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme","name":"reviewer","version":"1.2.3"},"requirements":{},"dependencies":[],"spec":{}}`)
	for name, metadata := range map[string][]byte{
		"duplicate key": []byte(`{"schema_version":1,"kind":"skill","kind":"bundle","artifact":{"namespace":"acme","name":"reviewer","version":"1.2.3"},"requirements":{},"dependencies":[],"spec":{}}`),
		"invalid UTF-8": append(append([]byte(nil), valid...), 0xff),
		"unknown field": []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme","name":"reviewer","version":"1.2.3"},"requirements":{},"dependencies":[],"spec":{},"extra":true}`),
		"oversized":     []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme","name":"reviewer","version":"1.2.3"},"requirements":{},"dependencies":[],"spec":{"value":"` + strings.Repeat("x", artifactdescriptor.MaxNormalizedMetadataBytes) + `"}}`),
	} {
		if _, err := artifactdescriptor.New(tenantID, versionID, shared.SHA256Digest(metadata), "application/vnd.thinkpixel.skill.manifest.v1+json", metadata); err == nil {
			t.Fatalf("%s: accepted invalid metadata", name)
		}
	}
	if _, err := artifactdescriptor.New(tenantID, versionID, shared.SHA256Digest(valid), "application/vnd.thinkpixel.bundle.manifest.v1+json", valid); err == nil {
		t.Fatal("accepted mismatched media type")
	}
	wrongDigest := shared.SHA256Digest([]byte("different"))
	if _, err := artifactdescriptor.New(tenantID, versionID, wrongDigest, "application/vnd.thinkpixel.skill.manifest.v1+json", valid); err == nil {
		t.Fatal("accepted mismatched descriptor digest")
	}
}
