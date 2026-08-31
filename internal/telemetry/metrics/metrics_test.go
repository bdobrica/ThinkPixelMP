package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRegistryUsesBoundedLabelsAndExpectedCollectors(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordOperation(OperationArtifactRegistration, OutcomeFailed, ReasonInvalid); err != nil {
		t.Fatal(err)
	}
	if err := r.ObserveDependency(DependencyRegistry, OutcomeSucceeded, 25*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := r.SetStaleEvidence(2); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCatalogEntries(3); err != nil {
		t.Fatal(err)
	}
	if err := r.SetOutboxLag(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDatabaseSaturation(.5); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordRevocation("critical"); err != nil {
		t.Fatal(err)
	}
	families, err := r.Gatherer().Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 7 {
		t.Fatalf("metric families=%d, want 7", len(families))
	}
	for _, family := range families {
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if strings.Contains(label.GetName(), "tenant") || strings.Contains(label.GetName(), "artifact") || strings.Contains(label.GetName(), "evidence") {
					t.Fatalf("identifier label exposed: %s", label.GetName())
				}
			}
		}
	}
	if got := testutil.ToFloat64(r.operations.WithLabelValues("artifact_registration", "failed", "invalid")); got != 1 {
		t.Fatalf("operations=%v", got)
	}
}

func TestRegistryRejectsUnboundedOrInvalidValues(t *testing.T) {
	r, _ := NewRegistry()
	for _, err := range []error{
		r.RecordOperation(Operation("tenant-secret-canary"), OutcomeFailed, ReasonInternal),
		r.RecordOperation(OperationAPI, OutcomeSucceeded, ReasonInternal),
		r.ObserveDependency(Dependency("https://secret.example"), OutcomeFailed, time.Second),
		r.ObserveDependency(DependencyRegistry, OutcomeFailed, -time.Second),
		r.SetStaleEvidence(-1), r.SetCatalogEntries(-1), r.SetOutboxLag(-time.Second),
		r.SetDatabaseSaturation(1.1), r.RecordRevocation("tenant-secret-canary"),
	} {
		if err == nil {
			t.Fatal("invalid metric value accepted")
		}
	}
	if families, err := r.Gatherer().Gather(); err != nil || len(families) != 4 {
		t.Fatalf("invalid values changed collectors: families=%d err=%v", len(families), err)
	}
}
