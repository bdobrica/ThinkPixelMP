package artifactversion

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxSemanticVersionBytes = 255
	MaxListSize             = 200
)

type Class string

const (
	ClassInstructional   Class = "instructional"
	ClassExecutableLocal Class = "executable-local"
	ClassRemoteService   Class = "remote-service"
	ClassComposite       Class = "composite"
)

type DeliveryModel string

const (
	DeliveryOCI            DeliveryModel = "oci"
	DeliveryRemote         DeliveryModel = "remote"
	DeliveryImportedSource DeliveryModel = "imported-source"
)

type Lifecycle string

const (
	LifecycleActive      Lifecycle = "active"
	LifecycleDeprecated  Lifecycle = "deprecated"
	LifecycleQuarantined Lifecycle = "quarantined"
	LifecycleRevoked     Lifecycle = "revoked"
)

type SemanticVersion string

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

func ParseSemanticVersion(value string) (SemanticVersion, error) {
	if len(value) < 5 || len(value) > MaxSemanticVersionBytes || !semanticVersionPattern.MatchString(value) {
		return "", fmt.Errorf("artifact version: invalid semantic version")
	}
	return SemanticVersion(value), nil
}

func (version SemanticVersion) String() string { return string(version) }

// ArtifactVersion binds one logical Artifact and publisher attribution to an
// exact immutable content digest and complete canonical semantic-version key.
type ArtifactVersion struct {
	tenantID     shared.UUID
	id           shared.UUID
	artifactID   shared.UUID
	publisherID  shared.UUID
	version      SemanticVersion
	digest       shared.Digest
	kind         artifact.Kind
	class        Class
	delivery     DeliveryModel
	lifecycle    Lifecycle
	registeredAt time.Time
}

// Repository is the tenant-scoped persistence boundary for ArtifactVersions.
type Repository interface {
	Create(context.Context, ArtifactVersion) error
	Get(context.Context, shared.UUID, shared.UUID) (ArtifactVersion, error)
	GetByDigest(context.Context, shared.UUID, shared.Digest) (ArtifactVersion, error)
	GetBySemanticVersion(context.Context, shared.UUID, shared.UUID, SemanticVersion) (ArtifactVersion, error)
	List(context.Context, shared.UUID, shared.UUID, *shared.UUID, int) ([]ArtifactVersion, error)
}

// New creates a newly registered active ArtifactVersion.
func New(tenantID, id, artifactID, publisherID shared.UUID, version SemanticVersion, digest shared.Digest, kind artifact.Kind, class Class, delivery DeliveryModel, registeredAt time.Time) (ArtifactVersion, error) {
	return restore(tenantID, id, artifactID, publisherID, version, digest, kind, class, delivery, LifecycleActive, registeredAt)
}

// Restore validates an ArtifactVersion loaded from an adapter.
func Restore(tenantID, id, artifactID, publisherID shared.UUID, version SemanticVersion, digest shared.Digest, kind artifact.Kind, class Class, delivery DeliveryModel, lifecycle Lifecycle, registeredAt time.Time) (ArtifactVersion, error) {
	return restore(tenantID, id, artifactID, publisherID, version, digest, kind, class, delivery, lifecycle, registeredAt)
}

func restore(tenantID, id, artifactID, publisherID shared.UUID, version SemanticVersion, digest shared.Digest, kind artifact.Kind, class Class, delivery DeliveryModel, lifecycle Lifecycle, registeredAt time.Time) (ArtifactVersion, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ID": id, "Artifact ID": artifactID, "Publisher ID": publisherID} {
		if _, err := identifier.MarshalText(); err != nil {
			return ArtifactVersion{}, fmt.Errorf("artifact version: %s: %w", label, err)
		}
	}
	parsedVersion, err := ParseSemanticVersion(version.String())
	if err != nil {
		return ArtifactVersion{}, err
	}
	if _, err := digest.MarshalText(); err != nil {
		return ArtifactVersion{}, fmt.Errorf("artifact version: digest: %w", err)
	}
	if !validCombination(kind, class, delivery) {
		return ArtifactVersion{}, fmt.Errorf("artifact version: invalid kind, class, or delivery model")
	}
	if !validLifecycle(lifecycle) {
		return ArtifactVersion{}, fmt.Errorf("artifact version: invalid lifecycle")
	}
	if registeredAt.IsZero() {
		return ArtifactVersion{}, fmt.Errorf("artifact version: registration time is required")
	}
	return ArtifactVersion{tenantID, id, artifactID, publisherID, parsedVersion, digest, kind, class, delivery, lifecycle, registeredAt.UTC()}, nil
}

func validCombination(kind artifact.Kind, class Class, delivery DeliveryModel) bool {
	if delivery == DeliveryImportedSource {
		return validKindClass(kind, class)
	}
	switch kind {
	case artifact.KindSkill:
		return (class == ClassInstructional || class == ClassExecutableLocal) && delivery == DeliveryOCI
	case artifact.KindAgentRuntime:
		return class == ClassExecutableLocal && delivery == DeliveryOCI
	case artifact.KindMCPServer:
		return (class == ClassExecutableLocal && delivery == DeliveryOCI) || (class == ClassRemoteService && delivery == DeliveryRemote)
	case artifact.KindRemoteAgent:
		return class == ClassRemoteService && delivery == DeliveryOCI
	case artifact.KindBundle:
		return class == ClassComposite && delivery == DeliveryOCI
	default:
		return false
	}
}

func validKindClass(kind artifact.Kind, class Class) bool {
	switch kind {
	case artifact.KindSkill:
		return class == ClassInstructional || class == ClassExecutableLocal
	case artifact.KindAgentRuntime:
		return class == ClassExecutableLocal
	case artifact.KindMCPServer:
		return class == ClassExecutableLocal || class == ClassRemoteService
	case artifact.KindRemoteAgent:
		return class == ClassRemoteService
	case artifact.KindBundle:
		return class == ClassComposite
	default:
		return false
	}
}

func validLifecycle(value Lifecycle) bool {
	switch value {
	case LifecycleActive, LifecycleDeprecated, LifecycleQuarantined, LifecycleRevoked:
		return true
	default:
		return false
	}
}

func (version ArtifactVersion) TenantID() shared.UUID            { return version.tenantID }
func (version ArtifactVersion) ID() shared.UUID                  { return version.id }
func (version ArtifactVersion) ArtifactID() shared.UUID          { return version.artifactID }
func (version ArtifactVersion) PublisherID() shared.UUID         { return version.publisherID }
func (version ArtifactVersion) SemanticVersion() SemanticVersion { return version.version }
func (version ArtifactVersion) Digest() shared.Digest            { return version.digest }
func (version ArtifactVersion) Kind() artifact.Kind              { return version.kind }
func (version ArtifactVersion) Class() Class                     { return version.class }
func (version ArtifactVersion) DeliveryModel() DeliveryModel     { return version.delivery }
func (version ArtifactVersion) Lifecycle() Lifecycle             { return version.lifecycle }
func (version ArtifactVersion) RegisteredAt() time.Time          { return version.registeredAt }
