package artifactrequirement_test

import (
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactrequirement"
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

func TestArtifactRequirementPreservesValidatedDeclaration(t *testing.T) {
	metadata := []byte(`{"schema_version":1,"capabilities":{"required":["scm.pull_request.read","scm.repository.read"],"optional":["scm.repository.write"]},"runtime":{"profile":"standard","minimum_isolation_class":"container-standard","os":"linux","architectures":["amd64","arm64"],"minimum_cpu_millicores":250,"minimum_memory_bytes":268435456,"durable_workspace_required":true,"gpu_required":false,"gpu_classes":[],"adapter":{"kind":"agentd","compatibility":"^1.0.0"}},"network":{"profile":"restricted-external","endpoint_classes":["package-mirror","source-control"]},"integrations":[{"kind":"model-feature","identifier":"tool-use","required":true}]}`)
	requirement, err := artifactrequirement.New(
		mustID(t, "0198fc21-ced5-7000-8000-000000000000"),
		mustID(t, "0198fc21-ced5-7000-8000-000000000001"),
		shared.SHA256Digest(metadata), metadata,
	)
	if err != nil || requirement.RequirementDigest() != shared.SHA256Digest(metadata) {
		t.Fatalf("requirement: %#v %v", requirement, err)
	}
	returned := requirement.NormalizedRequirement()
	returned[0] = '['
	if requirement.NormalizedRequirement()[0] != '{' {
		t.Fatal("normalized requirement was mutable through accessor")
	}
}

func TestArtifactRequirementRejectsInvalidDeclaration(t *testing.T) {
	tenantID := mustID(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := mustID(t, "0198fc21-ced5-7000-8000-000000000001")
	valid := []byte(`{"schema_version":1}`)
	for name, metadata := range map[string][]byte{
		"missing schema":           []byte(`{"capabilities":{}}`),
		"unknown privileged field": []byte(`{"schema_version":1,"runtime":{"service_account":"admin"}}`),
		"duplicate key":            []byte(`{"schema_version":1,"schema_version":1}`),
		"unsorted set":             []byte(`{"schema_version":1,"capabilities":{"required":["scm.repository.write","scm.repository.read"]}}`),
		"invalid capability":       []byte(`{"schema_version":1,"capabilities":{"required":["cluster-admin"]}}`),
		"missing endpoint classes": []byte(`{"schema_version":1,"network":{"profile":"restricted-external"}}`),
		"unexpected endpoints":     []byte(`{"schema_version":1,"network":{"profile":"none","endpoint_classes":["internet"]}}`),
		"invalid integration":      []byte(`{"schema_version":1,"integrations":[{"kind":"credential","identifier":"secret","required":true}]}`),
		"invalid UTF-8":            append(append([]byte(nil), valid...), 0xff),
		"oversized":                []byte(`{"schema_version":1,"integrations":[{"kind":"model-feature","identifier":"` + strings.Repeat("x", artifactrequirement.MaxNormalizedMetadataBytes) + `","required":true}]}`),
	} {
		if _, err := artifactrequirement.New(tenantID, versionID, shared.SHA256Digest(metadata), metadata); err == nil {
			t.Fatalf("%s: accepted invalid requirement", name)
		}
	}
	if _, err := artifactrequirement.New(tenantID, versionID, shared.SHA256Digest([]byte("different")), valid); err == nil {
		t.Fatal("accepted mismatched requirement digest")
	}
}
