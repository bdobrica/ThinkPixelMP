package artifactdependency_test

import (
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdependency"
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

func TestArtifactDependencyPreservesValidatedDeclaration(t *testing.T) {
	metadata := []byte(`{"schema_version":1,"name":"review-engine","artifact":"acme/platform/review-engine","required":false,"catalog":"production","source":"local-enterprise","selector":{"range":">=1.2.0 <2.0.0"}}`)
	dependency, err := artifactdependency.New(mustID(t, "0198fc21-ced5-7000-8000-000000000000"), mustID(t, "0198fc21-ced5-7000-8000-000000000001"), 7, shared.SHA256Digest(metadata), metadata)
	if err != nil || dependency.Index() != 7 || dependency.Name() != "review-engine" || dependency.Artifact().String() != "acme/platform/review-engine" || dependency.Required() || dependency.SelectorKind() != artifactdependency.SelectorRange || dependency.SelectorValue() != ">=1.2.0 <2.0.0" {
		t.Fatalf("dependency: %#v %v", dependency, err)
	}
	if catalog, ok := dependency.Catalog(); !ok || catalog != "production" {
		t.Fatalf("catalog = %q, %v", catalog, ok)
	}
	returned := dependency.NormalizedDependency()
	returned[0] = '['
	if dependency.NormalizedDependency()[0] != '{' {
		t.Fatal("normalized dependency was mutable through accessor")
	}
}

func TestArtifactDependencySelectors(t *testing.T) {
	tenantID := mustID(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := mustID(t, "0198fc21-ced5-7000-8000-000000000001")
	for _, metadata := range [][]byte{
		[]byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"digest":"sha256:` + strings.Repeat("a", 64) + `"}}`),
		[]byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"version":"1.2.3-rc.1+linux"}}`),
	} {
		if _, err := artifactdependency.New(tenantID, versionID, 0, shared.SHA256Digest(metadata), metadata); err != nil {
			t.Fatalf("valid selector rejected: %v", err)
		}
	}
}

func TestArtifactDependencyRejectsInvalidDeclaration(t *testing.T) {
	tenantID := mustID(t, "0198fc21-ced5-7000-8000-000000000000")
	versionID := mustID(t, "0198fc21-ced5-7000-8000-000000000001")
	valid := []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"range":"^1.0.0"}}`)
	for name, metadata := range map[string][]byte{
		"unknown field":        []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"range":"^1.0.0"},"authority":"admin"}`),
		"duplicate key":        []byte(`{"schema_version":1,"name":"engine","name":"other","artifact":"acme/engine","required":true,"selector":{"range":"^1.0.0"}}`),
		"missing required":     []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","selector":{"range":"^1.0.0"}}`),
		"invalid name":         []byte(`{"schema_version":1,"name":"Engine","artifact":"acme/engine","required":true,"selector":{"range":"^1.0.0"}}`),
		"invalid artifact":     []byte(`{"schema_version":1,"name":"engine","artifact":"engine","required":true,"selector":{"range":"^1.0.0"}}`),
		"empty catalog":        []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"catalog":"","selector":{"range":"^1.0.0"}}`),
		"null catalog":         []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"catalog":null,"selector":{"range":"^1.0.0"}}`),
		"empty source":         []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"source":"","selector":{"range":"^1.0.0"}}`),
		"multiple selectors":   []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"version":"1.0.0","range":"^1.0.0"}}`),
		"mutable latest":       []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"range":"latest"}}`),
		"invalid exact digest": []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"digest":"sha256:abc"}}`),
		"invalid version":      []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"selector":{"version":"v1.0.0"}}`),
		"invalid UTF-8":        append(append([]byte(nil), valid...), 0xff),
		"oversized":            []byte(`{"schema_version":1,"name":"engine","artifact":"acme/engine","required":true,"source":"` + strings.Repeat("x", artifactdependency.MaxNormalizedMetadataBytes) + `","selector":{"range":"^1.0.0"}}`),
	} {
		if _, err := artifactdependency.New(tenantID, versionID, 0, shared.SHA256Digest(metadata), metadata); err == nil {
			t.Fatalf("%s: accepted invalid dependency", name)
		}
	}
	if _, err := artifactdependency.New(tenantID, versionID, artifactdependency.MaxDependencies, shared.SHA256Digest(valid), valid); err == nil {
		t.Fatal("accepted out-of-range declaration index")
	}
	if _, err := artifactdependency.New(tenantID, versionID, 0, shared.SHA256Digest([]byte("different")), valid); err == nil {
		t.Fatal("accepted mismatched dependency digest")
	}
}
