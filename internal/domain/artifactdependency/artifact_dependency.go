package artifactdependency

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	SchemaVersion              = 1
	MaxDependencies            = 256
	MaxNormalizedMetadataBytes = 1 << 20
)

type SelectorKind string

const (
	SelectorDigest  SelectorKind = "digest"
	SelectorVersion SelectorKind = "version"
	SelectorRange   SelectorKind = "range"
)

// ArtifactDependency is one immutable, ordered dependency declaration. It is
// resolution input only and never grants the parent or child runtime authority.
type ArtifactDependency struct {
	tenantID          shared.UUID
	artifactVersionID shared.UUID
	index             uint16
	name              string
	artifact          shared.ArtifactReference
	required          bool
	catalog           string
	source            string
	selectorKind      SelectorKind
	selectorValue     string
	dependencyDigest  shared.Digest
	normalizedValue   []byte
}

type Repository interface {
	Create(context.Context, ArtifactDependency) error
	Get(context.Context, shared.UUID, shared.UUID, uint16) (ArtifactDependency, error)
	List(context.Context, shared.UUID, shared.UUID) ([]ArtifactDependency, error)
}

// New accepts normalized dependency bytes produced by the trusted descriptor
// validator and binds their separately supplied digest to the exact bytes.
func New(tenantID, artifactVersionID shared.UUID, index uint16, dependencyDigest shared.Digest, normalizedValue []byte) (ArtifactDependency, error) {
	return restore(tenantID, artifactVersionID, index, dependencyDigest, normalizedValue)
}

func Restore(tenantID, artifactVersionID shared.UUID, index uint16, dependencyDigest shared.Digest, normalizedValue []byte) (ArtifactDependency, error) {
	return restore(tenantID, artifactVersionID, index, dependencyDigest, normalizedValue)
}

type document struct {
	SchemaVersion int                        `json:"schema_version"`
	Name          string                     `json:"name"`
	Artifact      string                     `json:"artifact"`
	Required      *bool                      `json:"required"`
	Catalog       json.RawMessage            `json:"catalog"`
	Source        json.RawMessage            `json:"source"`
	Selector      map[string]json.RawMessage `json:"selector"`
}

func restore(tenantID, artifactVersionID shared.UUID, index uint16, dependencyDigest shared.Digest, normalizedValue []byte) (ArtifactDependency, error) {
	if index >= MaxDependencies {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid declaration index")
	}
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ArtifactVersion ID": artifactVersionID} {
		if _, err := identifier.MarshalText(); err != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: %s: %w", label, err)
		}
	}
	if _, err := dependencyDigest.MarshalText(); err != nil {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: digest: %w", err)
	}
	if len(normalizedValue) < 2 || len(normalizedValue) > MaxNormalizedMetadataBytes || !utf8.Valid(normalizedValue) || !json.Valid(normalizedValue) || hasDuplicateKeyOrTrailingContent(normalizedValue) {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid normalized metadata")
	}
	if shared.SHA256Digest(normalizedValue) != dependencyDigest {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: digest does not match normalized metadata")
	}
	decoder := json.NewDecoder(bytes.NewReader(normalizedValue))
	decoder.DisallowUnknownFields()
	var value document
	if err := decoder.Decode(&value); err != nil || value.SchemaVersion != SchemaVersion || value.Required == nil {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid V1 document")
	}
	if !validDNSToken(value.Name) {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid name")
	}
	artifact, err := shared.ParseArtifactReference(value.Artifact)
	if err != nil {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid artifact")
	}
	catalog := ""
	if value.Catalog != nil {
		if json.Unmarshal(value.Catalog, &catalog) != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid catalog")
		}
	}
	if value.Catalog != nil && !validDNSToken(catalog) {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid catalog")
	}
	source := ""
	if value.Source != nil {
		if json.Unmarshal(value.Source, &source) != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid source")
		}
	}
	if (value.Source != nil && (utf8.RuneCountInString(source) < 1 || utf8.RuneCountInString(source) > 255)) || len(value.Selector) != 1 {
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid source or selector")
	}
	var selectorKind SelectorKind
	var selectorValue string
	for key, raw := range value.Selector {
		if json.Unmarshal(raw, &selectorValue) != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid selector")
		}
		selectorKind = SelectorKind(key)
	}
	switch selectorKind {
	case SelectorDigest:
		if _, err := shared.ParseDigest(selectorValue); err != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid digest selector")
		}
	case SelectorVersion:
		if _, err := artifactversion.ParseSemanticVersion(selectorValue); err != nil {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid version selector")
		}
	case SelectorRange:
		if utf8.RuneCountInString(selectorValue) < 1 || utf8.RuneCountInString(selectorValue) > 255 || selectorValue == "latest" || selectorValue == "*" {
			return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid range selector")
		}
	default:
		return ArtifactDependency{}, fmt.Errorf("artifact dependency: invalid selector")
	}
	copyValue := append([]byte(nil), normalizedValue...)
	return ArtifactDependency{tenantID, artifactVersionID, index, value.Name, artifact, *value.Required, catalog, source, selectorKind, selectorValue, dependencyDigest, copyValue}, nil
}

func validDNSToken(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if index > 0 && index < len(value)-1 {
			valid = valid || character == '-'
		}
		if !valid {
			return false
		}
	}
	return true
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

func (value ArtifactDependency) TenantID() shared.UUID              { return value.tenantID }
func (value ArtifactDependency) ArtifactVersionID() shared.UUID     { return value.artifactVersionID }
func (value ArtifactDependency) Index() uint16                      { return value.index }
func (value ArtifactDependency) Name() string                       { return value.name }
func (value ArtifactDependency) Artifact() shared.ArtifactReference { return value.artifact }
func (value ArtifactDependency) Required() bool                     { return value.required }
func (value ArtifactDependency) Catalog() (string, bool)            { return value.catalog, value.catalog != "" }
func (value ArtifactDependency) Source() (string, bool)             { return value.source, value.source != "" }
func (value ArtifactDependency) SelectorKind() SelectorKind         { return value.selectorKind }
func (value ArtifactDependency) SelectorValue() string              { return value.selectorValue }
func (value ArtifactDependency) DependencyDigest() shared.Digest    { return value.dependencyDigest }
func (value ArtifactDependency) NormalizedDependency() []byte {
	return append([]byte(nil), value.normalizedValue...)
}
