package artifactsource

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const MaxReferenceBytes = 2048

type Kind string

const (
	KindOCI          Kind = "oci"
	KindRemote       Kind = "remote"
	KindImportRecord Kind = "import-record"
)

// ArtifactSource preserves the submitted source metadata and the exact source
// identity resolved before its ArtifactVersion was registered.
type ArtifactSource struct {
	tenantID           shared.UUID
	artifactVersionID  shared.UUID
	kind               Kind
	submittedReference string
	resolvedReference  string
	resolvedDigest     shared.Digest
	endpoint           string
	importRecordID     shared.UUID
	hasImportRecord    bool
}

// Repository is the tenant-scoped persistence boundary for ArtifactSource values.
// Repository is the tenant-scoped persistence boundary for artifact sources.
// Its operations join a transaction carried by context when supported.
type Repository interface {
	Create(context.Context, ArtifactSource) error
	Get(context.Context, shared.UUID, shared.UUID) (ArtifactSource, error)
}

func NewOCI(tenantID, artifactVersionID shared.UUID, submittedReference, resolvedReference string, resolvedDigest shared.Digest) (ArtifactSource, error) {
	return restore(tenantID, artifactVersionID, KindOCI, submittedReference, resolvedReference, resolvedDigest, "", nil)
}

func NewRemote(tenantID, artifactVersionID shared.UUID, descriptorReference, resolvedReference string, resolvedDigest shared.Digest, endpoint string) (ArtifactSource, error) {
	return restore(tenantID, artifactVersionID, KindRemote, descriptorReference, resolvedReference, resolvedDigest, endpoint, nil)
}

func NewImportRecord(tenantID, artifactVersionID, importRecordID shared.UUID, resolvedReference string, resolvedDigest shared.Digest) (ArtifactSource, error) {
	return restore(tenantID, artifactVersionID, KindImportRecord, "", resolvedReference, resolvedDigest, "", &importRecordID)
}

// Restore validates an ArtifactSource loaded from an adapter.
func Restore(tenantID, artifactVersionID shared.UUID, kind Kind, submittedReference, resolvedReference string, resolvedDigest shared.Digest, endpoint string, importRecordID *shared.UUID) (ArtifactSource, error) {
	return restore(tenantID, artifactVersionID, kind, submittedReference, resolvedReference, resolvedDigest, endpoint, importRecordID)
}

func restore(tenantID, artifactVersionID shared.UUID, kind Kind, submittedReference, resolvedReference string, resolvedDigest shared.Digest, endpoint string, importRecordID *shared.UUID) (ArtifactSource, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ArtifactVersion ID": artifactVersionID} {
		if _, err := identifier.MarshalText(); err != nil {
			return ArtifactSource{}, fmt.Errorf("artifact source: %s: %w", label, err)
		}
	}
	if _, err := resolvedDigest.MarshalText(); err != nil {
		return ArtifactSource{}, fmt.Errorf("artifact source: resolved digest: %w", err)
	}
	if err := requiredReference(resolvedReference); err != nil {
		return ArtifactSource{}, fmt.Errorf("artifact source: resolved reference: %w", err)
	}

	var importID shared.UUID
	hasImportRecord := importRecordID != nil
	if hasImportRecord {
		if _, err := importRecordID.MarshalText(); err != nil {
			return ArtifactSource{}, fmt.Errorf("artifact source: import record ID: %w", err)
		}
		importID = *importRecordID
	}

	switch kind {
	case KindOCI:
		if err := requiredReference(submittedReference); err != nil {
			return ArtifactSource{}, fmt.Errorf("artifact source: submitted reference: %w", err)
		}
		if !strings.HasSuffix(resolvedReference, "@"+resolvedDigest.String()) {
			return ArtifactSource{}, fmt.Errorf("artifact source: resolved OCI reference does not bind the resolved digest")
		}
		if endpoint != "" || hasImportRecord {
			return ArtifactSource{}, fmt.Errorf("artifact source: invalid OCI metadata")
		}
	case KindRemote:
		if err := requiredReference(submittedReference); err != nil {
			return ArtifactSource{}, fmt.Errorf("artifact source: descriptor reference: %w", err)
		}
		if err := httpsEndpoint(endpoint); err != nil {
			return ArtifactSource{}, fmt.Errorf("artifact source: endpoint: %w", err)
		}
		if hasImportRecord {
			return ArtifactSource{}, fmt.Errorf("artifact source: invalid remote metadata")
		}
	case KindImportRecord:
		if submittedReference != "" || endpoint != "" || !hasImportRecord {
			return ArtifactSource{}, fmt.Errorf("artifact source: invalid import-record metadata")
		}
	default:
		return ArtifactSource{}, fmt.Errorf("artifact source: invalid kind")
	}

	return ArtifactSource{tenantID, artifactVersionID, kind, submittedReference, resolvedReference, resolvedDigest, endpoint, importID, hasImportRecord}, nil
}

func requiredReference(value string) error {
	if len(value) < 1 || len(value) > MaxReferenceBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("invalid reference")
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return fmt.Errorf("invalid reference")
		}
	}
	return nil
}

func httpsEndpoint(value string) error {
	if err := requiredReference(value); err != nil {
		return err
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("HTTPS URL required")
	}
	return nil
}

func (source ArtifactSource) TenantID() shared.UUID          { return source.tenantID }
func (source ArtifactSource) ArtifactVersionID() shared.UUID { return source.artifactVersionID }
func (source ArtifactSource) Kind() Kind                     { return source.kind }
func (source ArtifactSource) SubmittedReference() string     { return source.submittedReference }
func (source ArtifactSource) ResolvedReference() string      { return source.resolvedReference }
func (source ArtifactSource) ResolvedDigest() shared.Digest  { return source.resolvedDigest }
func (source ArtifactSource) Endpoint() string               { return source.endpoint }
func (source ArtifactSource) ImportRecordID() (shared.UUID, bool) {
	return source.importRecordID, source.hasImportRecord
}
