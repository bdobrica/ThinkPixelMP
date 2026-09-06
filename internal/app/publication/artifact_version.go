package publication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/transaction"
)

var (
	registerArtifactVersionAction = mustReasonCode("artifact_version.register")
	artifactVersionResourceType   = mustReasonCode("artifact_version")
)

// RegisteredArtifactVersion is the immutable version and source identity
// established by the registration persistence seam.
type RegisteredArtifactVersion struct {
	Version artifactversion.ArtifactVersion
	Source  artifactsource.ArtifactSource
}

// ResolvedArtifactSource is trusted pipeline output. It is deliberately not a
// public request shape: later source adapters must resolve and inspect hostile
// input before invoking this seam.
type ResolvedArtifactSource struct {
	Kind               artifactsource.Kind `json:"kind"`
	SubmittedReference string              `json:"submitted_reference,omitempty"`
	ResolvedReference  string              `json:"resolved_reference"`
	ResolvedDigest     shared.Digest       `json:"resolved_digest"`
	Endpoint           string              `json:"endpoint,omitempty"`
	ImportRecordID     *shared.UUID        `json:"import_record_id,omitempty"`
}

// RegisterResolvedArtifactVersion contains only identity already resolved to
// immutable source data. Mutable tag resolution and descriptor validation are
// intentionally outside this PUB-005 skeleton.
type RegisterResolvedArtifactVersion struct {
	ArtifactID     shared.UUID                     `json:"artifact_id"`
	PublisherID    shared.UUID                     `json:"publisher_id"`
	Version        artifactversion.SemanticVersion `json:"version"`
	Digest         shared.Digest                   `json:"resolved_digest"`
	Class          artifactversion.Class           `json:"artifact_class"`
	DeliveryModel  artifactversion.DeliveryModel   `json:"delivery_model"`
	ResolvedSource ResolvedArtifactSource          `json:"resolved_source"`
}

type ArtifactVersionService struct {
	artifacts    artifact.Repository
	versions     artifactversion.Repository
	sources      artifactsource.Repository
	owners       NamespaceOwnerResolver
	idempotency  idempotency.Repository
	transactions transaction.Manager
	authorizer   authorization.Authorizer
	ids          IdentifierGenerator
	clock        clock.Clock
}

type ArtifactVersionDependencies struct {
	Artifacts    artifact.Repository
	Versions     artifactversion.Repository
	Sources      artifactsource.Repository
	Owners       NamespaceOwnerResolver
	Idempotency  idempotency.Repository
	Transactions transaction.Manager
	Authorizer   authorization.Authorizer
	IDs          IdentifierGenerator
	Clock        clock.Clock
}

func NewArtifactVersionService(dependencies ArtifactVersionDependencies) (*ArtifactVersionService, error) {
	if dependencies.Artifacts == nil || dependencies.Versions == nil || dependencies.Sources == nil ||
		dependencies.Owners == nil || dependencies.Idempotency == nil || dependencies.Transactions == nil ||
		dependencies.Authorizer == nil || dependencies.IDs == nil || dependencies.Clock == nil {
		return nil, errors.New("artifact version service: all dependencies are required")
	}
	return &ArtifactVersionService{dependencies.Artifacts, dependencies.Versions, dependencies.Sources,
		dependencies.Owners, dependencies.Idempotency, dependencies.Transactions, dependencies.Authorizer,
		dependencies.IDs, dependencies.Clock}, nil
}

// RegisterResolved atomically persists an ArtifactVersion and its immutable
// source identity. Only trusted resolution/inspection code may call this seam.
func (service *ArtifactVersionService) RegisterResolved(ctx context.Context, actor identity.Identity, key string, command RegisterResolvedArtifactVersion) (RegisteredArtifactVersion, error) {
	if err := service.authorizer.Authorize(ctx, actor, authorization.ActionPublish); err != nil {
		return RegisteredArtifactVersion{}, err
	}
	ctx, err := authenticatedAuditContext(ctx, actor)
	if err != nil {
		return RegisteredArtifactVersion{}, err
	}
	if err := validateResolvedRegistration(command); err != nil {
		return RegisteredArtifactVersion{}, err
	}
	canonical, err := json.Marshal(command)
	if err != nil {
		return RegisteredArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_request")
	}
	requestDigest := shared.SHA256Digest(canonical)
	if err := idempotency.ValidateOwnership(actor.PrincipalID, registerArtifactVersionAction, key); err != nil {
		return RegisteredArtifactVersion{}, typed(shared.ErrorInvalid, "artifact_version.invalid_idempotency_key")
	}

	var result RegisteredArtifactVersion
	err = service.transactions.WithinTransaction(ctx, actor.TenantID, func(transactionContext context.Context) error {
		now := service.clock.Now()
		recordID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "artifact_version.identifier_unavailable")
		}
		record, err := idempotency.New(actor.TenantID, recordID, actor.PrincipalID, registerArtifactVersionAction,
			key, requestDigest, now, now.Add(idempotency.MinimumRetention))
		if err != nil {
			return typed(shared.ErrorInvalid, "artifact_version.invalid_request")
		}
		stored, created, err := service.idempotency.Acquire(transactionContext, record)
		if err != nil {
			return err
		}
		if !created {
			return service.replayArtifactVersion(transactionContext, actor.TenantID, stored, &result)
		}

		logicalArtifact, err := service.artifacts.Get(transactionContext, actor.TenantID, command.ArtifactID)
		if err != nil {
			return err
		}
		owner, err := service.owners.ResolveOwner(transactionContext, actor.TenantID, logicalArtifact.Identity().Namespace())
		if err != nil {
			return err
		}
		if owner != command.PublisherID {
			return typed(shared.ErrorForbidden, "namespace.publication_not_authorized")
		}
		versionID, err := service.ids.New()
		if err != nil {
			return typed(shared.ErrorInternal, "artifact_version.identifier_unavailable")
		}
		registered, err := artifactversion.New(actor.TenantID, versionID, logicalArtifact.ID(), command.PublisherID,
			command.Version, command.Digest, logicalArtifact.Kind(), command.Class, command.DeliveryModel, now)
		if err != nil {
			return typed(shared.ErrorInvalid, "artifact_version.invalid_request")
		}
		source, err := resolvedSource(actor.TenantID, versionID, command.ResolvedSource)
		if err != nil {
			return typed(shared.ErrorInvalid, "artifact_version.invalid_source")
		}
		if err := service.versions.Create(transactionContext, registered); err != nil {
			return err
		}
		if err := service.sources.Create(transactionContext, source); err != nil {
			return err
		}
		result = RegisteredArtifactVersion{Version: registered, Source: source}
		established, err := idempotency.NewResult(http.StatusCreated, artifactVersionResourceType.String(), versionID.String())
		if err != nil {
			return typed(shared.ErrorInternal, "artifact_version.result_unavailable")
		}
		_, err = service.idempotency.Complete(transactionContext, actor.TenantID, actor.PrincipalID,
			registerArtifactVersionAction, key, requestDigest, established, now)
		return err
	})
	return result, err
}

func validateResolvedRegistration(command RegisterResolvedArtifactVersion) error {
	if _, err := command.ArtifactID.MarshalText(); err != nil {
		return typed(shared.ErrorInvalid, "artifact_version.invalid_request")
	}
	if _, err := command.PublisherID.MarshalText(); err != nil {
		return typed(shared.ErrorInvalid, "artifact_version.invalid_request")
	}
	if _, err := artifactversion.ParseSemanticVersion(command.Version.String()); err != nil {
		return typed(shared.ErrorInvalid, "artifact_version.invalid_request")
	}
	if _, err := command.Digest.MarshalText(); err != nil || command.ResolvedSource.ResolvedDigest != command.Digest {
		return typed(shared.ErrorInvalid, "artifact_version.digest_mismatch")
	}
	if !sourceMatchesDelivery(command.ResolvedSource.Kind, command.DeliveryModel) {
		return typed(shared.ErrorInvalid, "artifact_version.invalid_source")
	}
	return nil
}

func sourceMatchesDelivery(kind artifactsource.Kind, delivery artifactversion.DeliveryModel) bool {
	return kind == artifactsource.KindOCI && delivery == artifactversion.DeliveryOCI ||
		kind == artifactsource.KindRemote && delivery == artifactversion.DeliveryRemote ||
		kind == artifactsource.KindImportRecord && delivery == artifactversion.DeliveryImportedSource
}

func resolvedSource(tenantID, versionID shared.UUID, source ResolvedArtifactSource) (artifactsource.ArtifactSource, error) {
	return artifactsource.Restore(tenantID, versionID, source.Kind, source.SubmittedReference,
		source.ResolvedReference, source.ResolvedDigest, source.Endpoint, source.ImportRecordID)
}

func (service *ArtifactVersionService) replayArtifactVersion(ctx context.Context, tenantID shared.UUID, record idempotency.Record, destination *RegisteredArtifactVersion) error {
	established, ok := record.Result()
	if !ok {
		return typed(shared.ErrorConflict, "idempotency.in_progress")
	}
	resourceType, hasType := established.ResourceType()
	resourceID, hasID := established.ResourceID()
	if established.Status() != http.StatusCreated || !hasType || resourceType != artifactVersionResourceType || !hasID {
		return typed(shared.ErrorConflict, "idempotency.result_mismatch")
	}
	identifier, err := shared.ParseUUID(resourceID)
	if err != nil {
		return typed(shared.ErrorInternal, "artifact_version.result_unavailable")
	}
	version, err := service.versions.Get(ctx, tenantID, identifier)
	if err != nil {
		return err
	}
	source, err := service.sources.Get(ctx, tenantID, identifier)
	if err != nil {
		return err
	}
	*destination = RegisteredArtifactVersion{Version: version, Source: source}
	return nil
}
