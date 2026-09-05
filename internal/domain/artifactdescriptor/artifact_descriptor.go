package artifactdescriptor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	SchemaVersion              = 1
	MaxNormalizedMetadataBytes = 1 << 20
)

type ArtifactDescriptor struct {
	tenantID           shared.UUID
	artifactVersionID  shared.UUID
	descriptorDigest   shared.Digest
	mediaType          string
	kind               artifact.Kind
	identity           shared.ArtifactReference
	semanticVersion    artifactversion.SemanticVersion
	normalizedMetadata []byte
}

type Repository interface {
	Create(context.Context, ArtifactDescriptor) error
	Get(context.Context, shared.UUID, shared.UUID) (ArtifactDescriptor, error)
}

// New accepts normalized descriptor bytes produced by a trusted validator. It
// validates the common V1 envelope and binds the supplied digest to those exact
// bytes; kind-specific validation remains part of the descriptor substrate.
func New(tenantID, artifactVersionID shared.UUID, descriptorDigest shared.Digest, mediaType string, normalizedMetadata []byte) (ArtifactDescriptor, error) {
	return restore(tenantID, artifactVersionID, descriptorDigest, mediaType, normalizedMetadata)
}

// Restore validates an ArtifactDescriptor loaded from an adapter.
func Restore(tenantID, artifactVersionID shared.UUID, descriptorDigest shared.Digest, mediaType string, normalizedMetadata []byte) (ArtifactDescriptor, error) {
	return restore(tenantID, artifactVersionID, descriptorDigest, mediaType, normalizedMetadata)
}

type envelope struct {
	SchemaVersion int             `json:"schema_version"`
	Kind          artifact.Kind   `json:"kind"`
	Artifact      coordinates     `json:"artifact"`
	Requirements  json.RawMessage `json:"requirements"`
	Dependencies  json.RawMessage `json:"dependencies"`
	Spec          json.RawMessage `json:"spec"`
}

type coordinates struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

func restore(tenantID, artifactVersionID shared.UUID, descriptorDigest shared.Digest, mediaType string, normalizedMetadata []byte) (ArtifactDescriptor, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ArtifactVersion ID": artifactVersionID} {
		if _, err := identifier.MarshalText(); err != nil {
			return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: %s: %w", label, err)
		}
	}
	if _, err := descriptorDigest.MarshalText(); err != nil {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: digest: %w", err)
	}
	if len(normalizedMetadata) < 2 || len(normalizedMetadata) > MaxNormalizedMetadataBytes {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: normalized metadata exceeds bounds")
	}
	if !utf8.Valid(normalizedMetadata) || !json.Valid(normalizedMetadata) || hasDuplicateKeyOrTrailingContent(normalizedMetadata) {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid normalized metadata")
	}
	if shared.SHA256Digest(normalizedMetadata) != descriptorDigest {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: digest does not match normalized metadata")
	}
	decoder := json.NewDecoder(bytes.NewReader(normalizedMetadata))
	decoder.DisallowUnknownFields()
	var value envelope
	if err := decoder.Decode(&value); err != nil {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid V1 envelope")
	}
	if value.SchemaVersion != SchemaVersion || !validKind(value.Kind) || mediaType != mediaTypeFor(value.Kind) {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid schema version, kind, or media type")
	}
	identity, err := shared.ParseArtifactReference(value.Artifact.Namespace + "/" + value.Artifact.Name)
	if err != nil || identity.Namespace() != value.Artifact.Namespace {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid artifact coordinates")
	}
	semanticVersion, err := artifactversion.ParseSemanticVersion(value.Artifact.Version)
	if err != nil {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid artifact coordinates")
	}
	if jsonType(value.Requirements) != "object" || jsonType(value.Dependencies) != "array" || jsonType(value.Spec) != "object" {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid common metadata")
	}
	var dependencies []json.RawMessage
	if err := json.Unmarshal(value.Dependencies, &dependencies); err != nil || len(dependencies) > 256 {
		return ArtifactDescriptor{}, fmt.Errorf("artifact descriptor: invalid dependencies")
	}
	metadataCopy := append([]byte(nil), normalizedMetadata...)
	return ArtifactDescriptor{tenantID, artifactVersionID, descriptorDigest, mediaType, value.Kind, identity, semanticVersion, metadataCopy}, nil
}

func jsonType(value json.RawMessage) string {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return ""
	}
	switch decoded.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return ""
	}
}

func hasDuplicateKeyOrTrailingContent(value []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if duplicateJSONValue(decoder) {
		return true
	}
	_, err := decoder.Token()
	return err != io.EOF
}

func duplicateJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, keyOK := keyToken.(string)
			if err != nil || !keyOK {
				return true
			}
			if _, exists := seen[key]; exists {
				return true
			}
			seen[key] = struct{}{}
			if duplicateJSONValue(decoder) {
				return true
			}
		}
		closing, err := decoder.Token()
		return err != nil || closing != json.Delim('}')
	case '[':
		for decoder.More() {
			if duplicateJSONValue(decoder) {
				return true
			}
		}
		closing, err := decoder.Token()
		return err != nil || closing != json.Delim(']')
	default:
		return true
	}
}

func validKind(kind artifact.Kind) bool {
	return mediaTypeFor(kind) != ""
}

func mediaTypeFor(kind artifact.Kind) string {
	switch kind {
	case artifact.KindSkill:
		return "application/vnd.thinkpixel.skill.manifest.v1+json"
	case artifact.KindAgentRuntime:
		return "application/vnd.thinkpixel.agent-runtime.manifest.v1+json"
	case artifact.KindMCPServer:
		return "application/vnd.thinkpixel.mcp-server.manifest.v1+json"
	case artifact.KindRemoteAgent:
		return "application/vnd.thinkpixel.remote-agent.manifest.v1+json"
	case artifact.KindBundle:
		return "application/vnd.thinkpixel.bundle.manifest.v1+json"
	default:
		return ""
	}
}

func (descriptor ArtifactDescriptor) TenantID() shared.UUID { return descriptor.tenantID }
func (descriptor ArtifactDescriptor) ArtifactVersionID() shared.UUID {
	return descriptor.artifactVersionID
}
func (descriptor ArtifactDescriptor) DescriptorDigest() shared.Digest {
	return descriptor.descriptorDigest
}
func (descriptor ArtifactDescriptor) MediaType() string                  { return descriptor.mediaType }
func (descriptor ArtifactDescriptor) Kind() artifact.Kind                { return descriptor.kind }
func (descriptor ArtifactDescriptor) Identity() shared.ArtifactReference { return descriptor.identity }
func (descriptor ArtifactDescriptor) SemanticVersion() artifactversion.SemanticVersion {
	return descriptor.semanticVersion
}
func (descriptor ArtifactDescriptor) NormalizedMetadata() []byte {
	return append([]byte(nil), descriptor.normalizedMetadata...)
}
