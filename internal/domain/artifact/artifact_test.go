package artifact_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
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

func TestArtifactValidationAndDefensiveLabels(t *testing.T) {
	labels := map[string]string{"security/tier": "reviewed"}
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("offset", 3600))
	value, err := artifact.New(id(t, "0198fc21-ced5-7000-8000-000000000000"), id(t, "0198fc21-ced5-7000-8000-000000000001"), id(t, "0198fc21-ced5-7000-8000-000000000002"), "acme/security", "reviewer", artifact.KindSkill, "Reviewer", "Checks changes", "https://example.test/reviewer", "https://example.test/source", labels, now)
	if err != nil {
		t.Fatal(err)
	}
	labels["security/tier"] = "changed"
	returned := value.Labels()
	returned["security/tier"] = "also-changed"
	if value.Identity().String() != "acme/security/reviewer" || value.Labels()["security/tier"] != "reviewed" || value.CreatedAt().Location() != time.UTC {
		t.Fatal("artifact did not preserve validated values defensively")
	}
}

func TestArtifactRejectsInvalidContractValues(t *testing.T) {
	tenant := id(t, "0198fc21-ced5-7000-8000-000000000000")
	identifier := id(t, "0198fc21-ced5-7000-8000-000000000001")
	namespaceID := id(t, "0198fc21-ced5-7000-8000-000000000002")
	valid := func(namespacePath, name string, kind artifact.Kind, homepage string, labels map[string]string) error {
		_, err := artifact.New(tenant, identifier, namespaceID, namespacePath, name, kind, "", "", homepage, "", labels, time.Now())
		return err
	}
	for _, test := range []struct {
		name, namespacePath, artifactName string
		kind                              artifact.Kind
		homepage                          string
		labels                            map[string]string
	}{
		{"uppercase name", "acme", "Reviewer", artifact.KindSkill, "", nil},
		{"long name", "acme", strings.Repeat("a", artifact.MaxNameBytes+1), artifact.KindSkill, "", nil},
		{"invalid namespace", "acme//security", "reviewer", artifact.KindSkill, "", nil},
		{"unknown kind", "acme", "reviewer", "plugin", "", nil},
		{"relative URI", "acme", "reviewer", artifact.KindSkill, "/relative", nil},
		{"invalid label", "acme", "reviewer", artifact.KindSkill, "", map[string]string{"UPPER": "value"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if valid(test.namespacePath, test.artifactName, test.kind, test.homepage, test.labels) == nil {
				t.Fatal("accepted invalid Artifact")
			}
		})
	}
}
