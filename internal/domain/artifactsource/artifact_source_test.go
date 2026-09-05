package artifactsource_test

import (
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
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
	parsed, err := shared.ParseDigest("sha256:" + strings.Repeat(character, 64))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestArtifactSourceVariantsPreserveResolvedIdentity(t *testing.T) {
	tenantID := id(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := id(t, "0198fc21-ced5-7000-8000-000000000001")
	importID := id(t, "0198fc21-ced5-7000-8000-000000000002")
	resolvedDigest := digest(t, "a")

	oci, err := artifactsource.NewOCI(tenantID, versionID, "registry.example/acme/reviewer:1.2.3", "registry.example/acme/reviewer@"+resolvedDigest.String(), resolvedDigest)
	if err != nil || oci.SubmittedReference() == oci.ResolvedReference() || oci.ResolvedDigest() != resolvedDigest {
		t.Fatalf("OCI source: %#v %v", oci, err)
	}
	remote, err := artifactsource.NewRemote(tenantID, versionID, "https://agents.example/card.json", "https://objects.example/card/immutable", resolvedDigest, "https://agents.example/a2a")
	if err != nil || remote.Endpoint() != "https://agents.example/a2a" {
		t.Fatalf("remote source: %#v %v", remote, err)
	}
	imported, err := artifactsource.NewImportRecord(tenantID, versionID, importID, "mcp-registry:server/example@1.0.0", resolvedDigest)
	gotImportID, ok := imported.ImportRecordID()
	if err != nil || !ok || gotImportID != importID {
		t.Fatalf("import source: %#v %v", imported, err)
	}
}

func TestArtifactSourceRejectsInvalidMetadata(t *testing.T) {
	tenantID := id(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := id(t, "0198fc21-ced5-7000-8000-000000000001")
	resolvedDigest := digest(t, "b")
	validResolved := "registry.example/acme/reviewer@" + resolvedDigest.String()

	for name, build := range map[string]func() error{
		"blank submitted reference": func() error {
			_, err := artifactsource.NewOCI(tenantID, versionID, "", validResolved, resolvedDigest)
			return err
		},
		"mutable OCI resolved reference": func() error {
			_, err := artifactsource.NewOCI(tenantID, versionID, "registry.example/acme/reviewer:latest", "registry.example/acme/reviewer:latest", resolvedDigest)
			return err
		},
		"long reference": func() error {
			_, err := artifactsource.NewRemote(tenantID, versionID, "descriptor", strings.Repeat("x", artifactsource.MaxReferenceBytes+1), resolvedDigest, "https://agents.example/a2a")
			return err
		},
		"insecure endpoint": func() error {
			_, err := artifactsource.NewRemote(tenantID, versionID, "descriptor", "immutable", resolvedDigest, "http://agents.example/a2a")
			return err
		},
		"missing import ID": func() error {
			_, err := artifactsource.Restore(tenantID, versionID, artifactsource.KindImportRecord, "", "immutable", resolvedDigest, "", nil)
			return err
		},
	} {
		if err := build(); err == nil {
			t.Fatalf("%s: accepted invalid metadata", name)
		}
	}
}
