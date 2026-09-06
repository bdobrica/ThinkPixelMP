package publication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
)

const (
	DefaultArtifactPageSize = 50
	MaxArtifactPageSize     = 200
	MaxArtifactQueryBytes   = 512
	artifactCursorLifetime  = 15 * time.Minute
)

var (
	createArtifactAction = mustReasonCode("artifact.create")
	artifactResourceType = mustReasonCode("artifact")
)

// NamespaceOwnerResolver supplies the live longest-prefix publication owner.
type NamespaceOwnerResolver interface {
	ResolveOwner(context.Context, shared.UUID, string) (shared.UUID, error)
}

// ArtifactService coordinates the logical Artifact API. Version registration
// and immutable source identity are deliberately separate use cases.
type ArtifactService struct {
	artifacts    artifact.Repository
	namespaces   namespace.Repository
	owners       NamespaceOwnerResolver
	idempotency  idempotency.Repository
	transactions transaction.Manager
	authorizer   authorization.Authorizer
	ids          IdentifierGenerator
	clock        clock.Clock
	cursors      *shared.CursorCodec
}

type ArtifactDependencies struct {
	Artifacts    artifact.Repository
	Namespaces   namespace.Repository
	Owners       NamespaceOwnerResolver
	Idempotency  idempotency.Repository
	Transactions transaction.Manager
	Authorizer   authorization.Authorizer
	IDs          IdentifierGenerator
	Clock        clock.Clock
	Cursors      *shared.CursorCodec
}

func NewArtifactService(dependencies ArtifactDependencies) (*ArtifactService, error) {
	if dependencies.Artifacts == nil || dependencies.Namespaces == nil || dependencies.Owners == nil ||
		dependencies.Idempotency == nil || dependencies.Transactions == nil || dependencies.Authorizer == nil ||
		dependencies.IDs == nil || dependencies.Clock == nil || dependencies.Cursors == nil {
		return nil, errors.New("artifact service: all dependencies are required")
	}
	return &ArtifactService{dependencies.Artifacts, dependencies.Namespaces, dependencies.Owners,
		dependencies.Idempotency, dependencies.Transactions, dependencies.Authorizer,
		dependencies.IDs, dependencies.Clock, dependencies.Cursors}, nil
}

type CreateArtifact struct {
	NamespaceID shared.UUID       `json:"namespace_id"`
	Name        string            `json:"name"`
	Kind        artifact.Kind     `json:"kind"`
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (service *ArtifactService) Create(ctx context.Context, actor identity.Identity, key string, command CreateArtifact) (artifact.Artifact, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionPublish); err != nil {
		return artifact.Artifact{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return artifact.Artifact{}, err
	}
	if _, err := command.NamespaceID.MarshalText(); err != nil {
		return artifact.Artifact{}, typed(shared.ErrorInvalid, "artifact.invalid_request")
	}
	canonical, err := json.Marshal(command)
	if err != nil {
		return artifact.Artifact{}, typed(shared.ErrorInvalid, "artifact.invalid_request")
	}
	digest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, createArtifactAction, key); err != nil {
		return artifact.Artifact{}, typed(shared.ErrorInvalid, "artifact.invalid_idempotency_key")
	}

	var result artifact.Artifact
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		recordID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "artifact.identifier_unavailable")
		}
		record, err := idempotency.New(actor.TenantID, recordID, actor.PrincipalID, createArtifactAction, key,
			digest, now, now.Add(idempotency.MinimumRetention))
		if err != nil {
			return typed(shared.ErrorInvalid, "artifact.invalid_request")
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replay(transactionContext, actor.TenantID, stored, &result)
		}

		root, err := service.namespaces.Get(transactionContext, actor.TenantID, command.NamespaceID)
		if err != nil {
			return err
		}
		owner, err := service.owners.ResolveOwner(transactionContext, actor.TenantID, root.Path())
		if err != nil {
			return err
		}
		if owner != root.OwnerPublisherID() {
			return typed(shared.ErrorForbidden, "namespace.publication_not_authorized")
		}
		artifactID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "artifact.identifier_unavailable")
		}
		result, err = artifact.New(actor.TenantID, artifactID, root.ID(), root.Path(), command.Name, command.Kind,
			command.DisplayName, command.Description, command.Homepage, command.Repository, command.Labels, now)
		if err != nil {
			return typed(shared.ErrorInvalid, "artifact.invalid_request")
		}
		if err := service.artifacts.Create(transactionContext, result); err != nil {
			return err
		}
		established, err := idempotency.NewResult(http.StatusCreated, artifactResourceType.String(), result.ID().String())
		if err != nil {
			return typed(shared.ErrorInternal, "artifact.result_unavailable")
		}
		_, err = service.idempotency.Complete(transactionContext, actor.TenantID, actor.PrincipalID,
			createArtifactAction, key, digest, established, now)
		return err
	})
	return result, err
}

func (service *ArtifactService) replay(ctx context.Context, tenantID shared.UUID, record idempotency.Record, destination *artifact.Artifact) error {
	established, ok := record.Result()
	if !ok {
		return typed(shared.ErrorConflict, "idempotency.in_progress")
	}
	resourceType, hasType := established.ResourceType()
	resourceID, hasID := established.ResourceID()
	if established.Status() != http.StatusCreated || !hasType || resourceType != artifactResourceType || !hasID {
		return typed(shared.ErrorConflict, "idempotency.result_mismatch")
	}
	identifier, err := shared.ParseUUID(resourceID)
	if err != nil {
		return typed(shared.ErrorInternal, "artifact.result_unavailable")
	}
	*destination, err = service.artifacts.Get(ctx, tenantID, identifier)
	return err
}

func (service *ArtifactService) Get(ctx context.Context, actor identity.Identity, artifactID shared.UUID) (artifact.Artifact, error) {
	if err := validateIdentity(actor); err != nil {
		return artifact.Artifact{}, err
	}
	return service.artifacts.Get(ctx, actor.TenantID, artifactID)
}

type ArtifactPage struct {
	Items      []artifact.Artifact
	NextCursor string
}

func (service *ArtifactService) List(ctx context.Context, actor identity.Identity, query string, pageSize int, encodedCursor string) (ArtifactPage, error) {
	if err := validateIdentity(actor); err != nil {
		return ArtifactPage{}, err
	}
	if !validArtifactQuery(query) {
		return ArtifactPage{}, typed(shared.ErrorInvalid, "artifact.invalid_query")
	}
	query = strings.TrimSpace(query)
	if pageSize < 1 || pageSize > MaxArtifactPageSize {
		return ArtifactPage{}, typed(shared.ErrorInvalid, "artifact.invalid_page_size")
	}
	queryDigest := shared.SHA256Digest([]byte(`{"query":` + string(mustJSON(query)) + `,"sort":"artifact_id"}`))
	scope := shared.CursorScope{TenantID: actor.TenantID, Endpoint: "/v1/artifacts", QueryDigest: queryDigest, PageSize: pageSize}
	var after *shared.UUID
	if encodedCursor != "" {
		cursor, err := service.cursors.Decode(encodedCursor, scope)
		if err != nil {
			return ArtifactPage{}, typed(shared.ErrorInvalid, "artifact.invalid_cursor")
		}
		position, err := shared.ParseUUID(cursor.Position)
		if err != nil {
			return ArtifactPage{}, typed(shared.ErrorInvalid, "artifact.invalid_cursor")
		}
		after = &position
	}
	values, err := service.artifacts.List(ctx, actor.TenantID, query, after, pageSize+1)
	if err != nil {
		return ArtifactPage{}, err
	}
	page := ArtifactPage{Items: values}
	if len(values) > pageSize {
		page.Items = values[:pageSize]
		page.NextCursor, err = service.cursors.Encode(scope, shared.Cursor{
			Position: page.Items[len(page.Items)-1].ID().String(), ExpiresAt: service.clock.Now().Add(artifactCursorLifetime),
		})
		if err != nil {
			return ArtifactPage{}, typed(shared.ErrorInternal, "artifact.cursor_unavailable")
		}
	}
	return page, nil
}

func validArtifactQuery(value string) bool {
	if len(value) > MaxArtifactQueryBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func mustJSON(value string) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}
