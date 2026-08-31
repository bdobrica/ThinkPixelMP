// Package metrics owns ThinkPixelMP's private Prometheus registry and its
// bounded, low-cardinality metric vocabulary.
package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Operation is a fixed operation family. It deliberately excludes resource
// identifiers and caller-controlled names.
type Operation string

const (
	OperationArtifactRegistration Operation = "artifact_registration"
	OperationEvidenceIngestion    Operation = "evidence_ingestion"
	OperationPromotion            Operation = "promotion"
	OperationResolution           Operation = "resolution"
	OperationImport               Operation = "import"
	OperationRemoteVerification   Operation = "remote_verification"
	OperationAPI                  Operation = "api"
)

// Outcome is the bounded result of an operation or dependency call.
type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeCancelled Outcome = "cancelled"
)

// Dependency identifies a replaceable external dependency class, never a
// destination, URL, tenant, or provider instance.
type Dependency string

const (
	DependencyRegistry  Dependency = "registry"
	DependencySignature Dependency = "signature"
	DependencyPolicy    Dependency = "policy"
	DependencyDatabase  Dependency = "database"
	DependencyEvidence  Dependency = "evidence_producer"
)

// ReasonFamily is an intentionally small failure classification.
type ReasonFamily string

const (
	ReasonNone         ReasonFamily = "none"
	ReasonInvalid      ReasonFamily = "invalid"
	ReasonUnavailable  ReasonFamily = "unavailable"
	ReasonUnauthorized ReasonFamily = "unauthorized"
	ReasonConflict     ReasonFamily = "conflict"
	ReasonInternal     ReasonFamily = "internal"
)

// Registry contains process-local collectors. It does not use Prometheus's
// global registry, so tests and multiple process components cannot mutate one
// another's collector set.
type Registry struct {
	registry           *prometheus.Registry
	operations         *prometheus.CounterVec
	dependencyDuration *prometheus.HistogramVec
	staleEvidence      prometheus.Gauge
	catalogEntries     prometheus.Gauge
	outboxLag          prometheus.Gauge
	databaseSaturation prometheus.Gauge
	revocations        *prometheus.CounterVec
}

// NewRegistry creates and fully registers the baseline collector set.
func NewRegistry() (*Registry, error) {
	r := &Registry{registry: prometheus.NewRegistry()}
	r.operations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "thinkpixelmp_operations_total", Help: "Completed marketplace operations by bounded result."}, []string{"operation", "outcome", "reason_family"})
	r.dependencyDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "thinkpixelmp_dependency_duration_seconds", Help: "External dependency call duration by dependency class and bounded result.", Buckets: prometheus.DefBuckets}, []string{"dependency", "outcome"})
	r.staleEvidence = prometheus.NewGauge(prometheus.GaugeOpts{Name: "thinkpixelmp_stale_evidence", Help: "Current number of stale evidence records."})
	r.catalogEntries = prometheus.NewGauge(prometheus.GaugeOpts{Name: "thinkpixelmp_catalog_entries", Help: "Current number of catalog entries."})
	r.outboxLag = prometheus.NewGauge(prometheus.GaugeOpts{Name: "thinkpixelmp_outbox_lag_seconds", Help: "Age in seconds of the oldest pending outbox record."})
	r.databaseSaturation = prometheus.NewGauge(prometheus.GaugeOpts{Name: "thinkpixelmp_database_pool_saturation_ratio", Help: "Database connections in use divided by the configured maximum."})
	r.revocations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "thinkpixelmp_revocations_total", Help: "Recorded digest revocations by bounded severity."}, []string{"severity"})
	if err := r.registry.Register(r.operations); err != nil {
		return nil, fmt.Errorf("register operations metric: %w", err)
	}
	if err := r.registry.Register(r.dependencyDuration); err != nil {
		return nil, fmt.Errorf("register dependency duration metric: %w", err)
	}
	for name, collector := range map[string]prometheus.Collector{"stale evidence": r.staleEvidence, "catalog entries": r.catalogEntries, "outbox lag": r.outboxLag, "database saturation": r.databaseSaturation, "revocations": r.revocations} {
		if err := r.registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register %s metric: %w", name, err)
		}
	}
	return r, nil
}

// Gatherer exposes the registry for the future /metrics HTTP adapter.
func (r *Registry) Gatherer() prometheus.Gatherer { return r.registry }

func (r *Registry) RecordOperation(operation Operation, outcome Outcome, reason ReasonFamily) error {
	if !validOperation(operation) || !validOutcome(outcome) || !validReason(reason) {
		return fmt.Errorf("invalid metric label")
	}
	if outcome == OutcomeSucceeded && reason != ReasonNone {
		return fmt.Errorf("successful operation must use reason none")
	}
	r.operations.WithLabelValues(string(operation), string(outcome), string(reason)).Inc()
	return nil
}

func (r *Registry) ObserveDependency(dependency Dependency, outcome Outcome, duration time.Duration) error {
	if !validDependency(dependency) || !validOutcome(outcome) || duration < 0 {
		return fmt.Errorf("invalid dependency observation")
	}
	r.dependencyDuration.WithLabelValues(string(dependency), string(outcome)).Observe(duration.Seconds())
	return nil
}

func (r *Registry) SetStaleEvidence(count int64) error {
	if count < 0 {
		return fmt.Errorf("stale evidence count must not be negative")
	}
	r.staleEvidence.Set(float64(count))
	return nil
}
func (r *Registry) SetCatalogEntries(count int64) error {
	if count < 0 {
		return fmt.Errorf("catalog entry count must not be negative")
	}
	r.catalogEntries.Set(float64(count))
	return nil
}
func (r *Registry) SetOutboxLag(lag time.Duration) error {
	if lag < 0 {
		return fmt.Errorf("outbox lag must not be negative")
	}
	r.outboxLag.Set(lag.Seconds())
	return nil
}
func (r *Registry) SetDatabaseSaturation(ratio float64) error {
	if ratio < 0 || ratio > 1 {
		return fmt.Errorf("database saturation must be between zero and one")
	}
	r.databaseSaturation.Set(ratio)
	return nil
}
func (r *Registry) RecordRevocation(severity string) error {
	if severity != "low" && severity != "medium" && severity != "high" && severity != "critical" {
		return fmt.Errorf("invalid revocation severity")
	}
	r.revocations.WithLabelValues(severity).Inc()
	return nil
}

func validOperation(v Operation) bool {
	return v == OperationArtifactRegistration || v == OperationEvidenceIngestion || v == OperationPromotion || v == OperationResolution || v == OperationImport || v == OperationRemoteVerification || v == OperationAPI
}
func validOutcome(v Outcome) bool {
	return v == OutcomeSucceeded || v == OutcomeFailed || v == OutcomeCancelled
}
func validDependency(v Dependency) bool {
	return v == DependencyRegistry || v == DependencySignature || v == DependencyPolicy || v == DependencyDatabase || v == DependencyEvidence
}
func validReason(v ReasonFamily) bool {
	return v == ReasonNone || v == ReasonInvalid || v == ReasonUnavailable || v == ReasonUnauthorized || v == ReasonConflict || v == ReasonInternal
}
