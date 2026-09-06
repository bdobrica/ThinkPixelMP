package artifact

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	MaxNameBytes        = 63
	MaxDisplayNameBytes = 256
	MaxDescriptionBytes = 4096
	MaxURIBytes         = 2048
	MaxLabels           = 64
	MaxLabelKeyBytes    = 128
	MaxLabelValueBytes  = 512
	MaxListSize         = 200
)

type Kind string

const (
	KindSkill        Kind = "skill"
	KindAgentRuntime Kind = "agent-runtime"
	KindMCPServer    Kind = "mcp-server"
	KindRemoteAgent  Kind = "remote-agent"
	KindBundle       Kind = "bundle"
)

var (
	namePattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	labelKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]*[a-z0-9])?$`)
)

// Artifact is a tenant-scoped logical identity. Its kind is fixed at creation;
// versions and content-addressed identity belong to ArtifactVersion.
type Artifact struct {
	tenantID    shared.UUID
	id          shared.UUID
	namespaceID shared.UUID
	identity    shared.ArtifactReference
	kind        Kind
	displayName string
	description string
	homepage    string
	repository  string
	labels      map[string]string
	createdAt   time.Time
}

// Repository is the tenant-scoped persistence boundary for logical Artifacts.
// Its operations join a transaction carried by context when supported.
type Repository interface {
	Create(context.Context, Artifact) error
	Get(context.Context, shared.UUID, shared.UUID) (Artifact, error)
	GetByIdentity(context.Context, shared.UUID, shared.ArtifactReference) (Artifact, error)
	List(context.Context, shared.UUID, *shared.UUID, int) ([]Artifact, error)
}

func New(tenantID, id, namespaceID shared.UUID, namespacePath, name string, kind Kind, displayName, description, homepage, repository string, labels map[string]string, createdAt time.Time) (Artifact, error) {
	return restore(tenantID, id, namespaceID, namespacePath, name, kind, displayName, description, homepage, repository, labels, createdAt)
}

// Restore validates an Artifact loaded from an adapter.
func Restore(tenantID, id, namespaceID shared.UUID, namespacePath, name string, kind Kind, displayName, description, homepage, repository string, labels map[string]string, createdAt time.Time) (Artifact, error) {
	return restore(tenantID, id, namespaceID, namespacePath, name, kind, displayName, description, homepage, repository, labels, createdAt)
}

func restore(tenantID, id, namespaceID shared.UUID, namespacePath, name string, kind Kind, displayName, description, homepage, repository string, labels map[string]string, createdAt time.Time) (Artifact, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ID": id, "Namespace ID": namespaceID} {
		if _, err := identifier.MarshalText(); err != nil {
			return Artifact{}, fmt.Errorf("artifact: %s: %w", label, err)
		}
	}
	if err := ValidateName(name); err != nil {
		return Artifact{}, fmt.Errorf("artifact: invalid name")
	}
	identity, err := shared.ParseArtifactReference(namespacePath + "/" + name)
	if err != nil || identity.Namespace() != namespacePath {
		return Artifact{}, fmt.Errorf("artifact: invalid namespace path")
	}
	if !validKind(kind) {
		return Artifact{}, fmt.Errorf("artifact: invalid kind")
	}
	if err := optionalText(displayName, MaxDisplayNameBytes); err != nil {
		return Artifact{}, fmt.Errorf("artifact: display name: %w", err)
	}
	if err := optionalText(description, MaxDescriptionBytes); err != nil {
		return Artifact{}, fmt.Errorf("artifact: description: %w", err)
	}
	if err := optionalURI(homepage); err != nil {
		return Artifact{}, fmt.Errorf("artifact: homepage: %w", err)
	}
	if err := optionalURI(repository); err != nil {
		return Artifact{}, fmt.Errorf("artifact: repository: %w", err)
	}
	labels, err = validatedLabels(labels)
	if err != nil {
		return Artifact{}, fmt.Errorf("artifact: labels: %w", err)
	}
	if createdAt.IsZero() {
		return Artifact{}, fmt.Errorf("artifact: creation time is required")
	}
	return Artifact{tenantID, id, namespaceID, identity, kind, displayName, description, homepage, repository, labels, createdAt.UTC()}, nil
}

func ValidateName(value string) error {
	if len(value) > MaxNameBytes || !namePattern.MatchString(value) {
		return fmt.Errorf("invalid name")
	}
	return nil
}

func validKind(kind Kind) bool {
	switch kind {
	case KindSkill, KindAgentRuntime, KindMCPServer, KindRemoteAgent, KindBundle:
		return true
	default:
		return false
	}
}

func optionalText(value string, maximum int) error {
	if value == "" {
		return nil
	}
	if len(value) > maximum || !utf8.ValidString(value) {
		return fmt.Errorf("invalid length or encoding")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("control character")
		}
	}
	return nil
}

func optionalURI(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > MaxURIBytes || !utf8.ValidString(value) {
		return fmt.Errorf("invalid length or encoding")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("absolute URI required")
	}
	return nil
}

func validatedLabels(labels map[string]string) (map[string]string, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("too many labels")
	}
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		if len(key) > MaxLabelKeyBytes || !labelKeyPattern.MatchString(key) || len(value) > MaxLabelValueBytes || !utf8.ValidString(value) {
			return nil, fmt.Errorf("invalid label")
		}
		copy[key] = value
	}
	return copy, nil
}

func (artifact Artifact) TenantID() shared.UUID              { return artifact.tenantID }
func (artifact Artifact) ID() shared.UUID                    { return artifact.id }
func (artifact Artifact) NamespaceID() shared.UUID           { return artifact.namespaceID }
func (artifact Artifact) Identity() shared.ArtifactReference { return artifact.identity }
func (artifact Artifact) Name() string                       { return artifact.identity.Name() }
func (artifact Artifact) Kind() Kind                         { return artifact.kind }
func (artifact Artifact) DisplayName() string                { return artifact.displayName }
func (artifact Artifact) Description() string                { return artifact.description }
func (artifact Artifact) Homepage() string                   { return artifact.homepage }
func (artifact Artifact) Repository() string                 { return artifact.repository }
func (artifact Artifact) CreatedAt() time.Time               { return artifact.createdAt }
func (artifact Artifact) Labels() map[string]string          { return cloneLabels(artifact.labels) }
func cloneLabels(labels map[string]string) map[string]string {
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	return copy
}
