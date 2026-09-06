package publication_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/security"
)

func TestArtifactVersionRegisterResolvedIsOwnedAtomicAndReplaySafe(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	logical := newLogicalArtifact(t, tenant)
	versions := &artifactVersionMemory{}
	sources := &artifactSourceMemory{}
	records := &idempotencyMemory{}
	transactions := &transactionMemory{}
	service := newArtifactVersionService(t, &artifactMemory{values: []artifact.Artifact{logical}}, versions, sources,
		ownerResolver{owner: owner}, records, transactions, []authorization.Grant{{
			TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin,
		}})
	actor := identity.Identity{TenantID: tenant, PrincipalID: "publisher"}
	command := resolvedOCICommand(t, logical.ID(), owner)

	created, err := service.RegisterResolved(context.Background(), actor, "registration-1", command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.RegisterResolved(context.Background(), actor, "registration-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if created.Version.ID() != replayed.Version.ID() || created.Version.Digest() != command.Digest ||
		created.Source.ResolvedDigest() != command.Digest || versions.creates != 1 || sources.creates != 1 ||
		versions.auditPrincipal != actor.PrincipalID || sources.auditPrincipal != actor.PrincipalID || transactions.calls != 2 {
		t.Fatalf("registration/replay mismatch: %#v %#v version creates=%d source creates=%d transactions=%d",
			created, replayed, versions.creates, sources.creates, transactions.calls)
	}

	changed := command
	changed.Version = mustSemanticVersion(t, "1.2.4")
	_, err = service.RegisterResolved(context.Background(), actor, "registration-1", changed)
	assertTyped(t, err, shared.ErrorConflict, "idempotency.request_mismatch")
}

func TestArtifactVersionRegisterResolvedRequiresExactRoleAndLiveOwner(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	other := parseID(t, "01991e80-2c00-7000-8000-000000000099")
	logical := newLogicalArtifact(t, tenant)
	command := resolvedOCICommand(t, logical.ID(), owner)

	unauthorizedVersions := &artifactVersionMemory{}
	unauthorized := newArtifactVersionService(t, &artifactMemory{values: []artifact.Artifact{logical}}, unauthorizedVersions,
		&artifactSourceMemory{}, ownerResolver{owner: owner}, &idempotencyMemory{}, &transactionMemory{},
		[]authorization.Grant{{TenantID: tenant, PrincipalID: "admin", Role: authorization.RoleNamespaceAdmin}})
	_, err := unauthorized.RegisterResolved(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "admin"}, "registration-1", command)
	assertTyped(t, err, shared.ErrorForbidden, "authorization.denied")
	if unauthorizedVersions.creates != 0 {
		t.Fatal("unauthorized registration reached persistence")
	}

	wrongOwnerVersions := &artifactVersionMemory{}
	wrongOwner := newArtifactVersionService(t, &artifactMemory{values: []artifact.Artifact{logical}}, wrongOwnerVersions,
		&artifactSourceMemory{}, ownerResolver{owner: other}, &idempotencyMemory{}, &transactionMemory{},
		[]authorization.Grant{{TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin}})
	_, err = wrongOwner.RegisterResolved(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "publisher"}, "registration-1", command)
	assertTyped(t, err, shared.ErrorForbidden, "namespace.publication_not_authorized")
	if wrongOwnerVersions.creates != 0 {
		t.Fatal("ownership failure reached persistence")
	}
}

func TestArtifactVersionRegisterResolvedRejectsMutableOrMismatchedSourceIdentity(t *testing.T) {
	tenant := parseID(t, "01991e80-2c00-7000-8000-000000000001")
	owner := parseID(t, "01991e80-2c00-7000-8000-000000000002")
	logical := newLogicalArtifact(t, tenant)
	versions := &artifactVersionMemory{}
	service := newArtifactVersionService(t, &artifactMemory{values: []artifact.Artifact{logical}}, versions,
		&artifactSourceMemory{}, ownerResolver{owner: owner}, &idempotencyMemory{}, &transactionMemory{},
		[]authorization.Grant{{TenantID: tenant, PrincipalID: "publisher", Role: authorization.RolePublicationAdmin}})
	actor := identity.Identity{TenantID: tenant, PrincipalID: "publisher"}

	mismatch := resolvedOCICommand(t, logical.ID(), owner)
	mismatch.ResolvedSource.ResolvedDigest = mustDigest(t, "sha256:"+string(bytes.Repeat([]byte{'b'}, 64)))
	_, err := service.RegisterResolved(context.Background(), actor, "registration-1", mismatch)
	assertTyped(t, err, shared.ErrorInvalid, "artifact_version.digest_mismatch")

	mutable := resolvedOCICommand(t, logical.ID(), owner)
	mutable.ResolvedSource.ResolvedReference = "registry.example/acme/reviewer:latest"
	_, err = service.RegisterResolved(context.Background(), actor, "registration-2", mutable)
	assertTyped(t, err, shared.ErrorInvalid, "artifact_version.invalid_source")
	if versions.creates != 0 {
		t.Fatal("invalid immutable source reached persistence")
	}
}

func newArtifactVersionService(t *testing.T, artifacts *artifactMemory, versions *artifactVersionMemory,
	sources *artifactSourceMemory, owners ownerResolver, records *idempotencyMemory, transactions *transactionMemory,
	grants []authorization.Grant,
) *publication.ArtifactVersionService {
	t.Helper()
	authorizer, err := security.NewAuthorizer(grants)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := shared.NewUUIDGenerator(clock.Fixed{Time: fixedNow}, bytes.NewReader(bytes.Repeat([]byte{41}, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	service, err := publication.NewArtifactVersionService(publication.ArtifactVersionDependencies{
		Artifacts: artifacts, Versions: versions, Sources: sources, Owners: owners, Idempotency: records,
		Transactions: transactions, Authorizer: authorizer, IDs: ids, Clock: clock.Fixed{Time: fixedNow},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func newLogicalArtifact(t *testing.T, tenant shared.UUID) artifact.Artifact {
	t.Helper()
	value, err := artifact.New(tenant, parseID(t, "01991e80-2c00-7000-8000-000000000003"),
		parseID(t, "01991e80-2c00-7000-8000-000000000004"), "acme/security", "reviewer",
		artifact.KindSkill, "", "", "", "", nil, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func resolvedOCICommand(t *testing.T, artifactID, publisherID shared.UUID) publication.RegisterResolvedArtifactVersion {
	t.Helper()
	digest := mustDigest(t, "sha256:"+string(bytes.Repeat([]byte{'a'}, 64)))
	return publication.RegisterResolvedArtifactVersion{
		ArtifactID: artifactID, PublisherID: publisherID, Version: mustSemanticVersion(t, "1.2.3+build.4"),
		Digest: digest, Class: artifactversion.ClassInstructional, DeliveryModel: artifactversion.DeliveryOCI,
		ResolvedSource: publication.ResolvedArtifactSource{Kind: artifactsource.KindOCI,
			SubmittedReference: "registry.example/acme/reviewer:1.2.3", ResolvedReference: "registry.example/acme/reviewer@" + digest.String(), ResolvedDigest: digest},
	}
}

func mustDigest(t *testing.T, value string) shared.Digest {
	t.Helper()
	digest, err := shared.ParseDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func mustSemanticVersion(t *testing.T, value string) artifactversion.SemanticVersion {
	t.Helper()
	version, err := artifactversion.ParseSemanticVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

type artifactVersionMemory struct {
	values         []artifactversion.ArtifactVersion
	creates        int
	auditPrincipal string
}

func (memory *artifactVersionMemory) Create(ctx context.Context, value artifactversion.ArtifactVersion) error {
	actor, ok := audit.ActorFromContext(ctx)
	if !ok {
		return errors.New("audit actor missing")
	}
	memory.auditPrincipal = actor.PrincipalID()
	for _, stored := range memory.values {
		if stored.TenantID() == value.TenantID() && (stored.Digest() == value.Digest() ||
			stored.ArtifactID() == value.ArtifactID() && stored.SemanticVersion() == value.SemanticVersion()) {
			return typed(shared.ErrorConflict, "artifact_version.conflict")
		}
	}
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}

func (memory *artifactVersionMemory) Get(_ context.Context, tenantID, id shared.UUID) (artifactversion.ArtifactVersion, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ID() == id {
			return value, nil
		}
	}
	return artifactversion.ArtifactVersion{}, typed(shared.ErrorNotFound, "artifact_version.not_found")
}
func (*artifactVersionMemory) GetByDigest(context.Context, shared.UUID, shared.Digest) (artifactversion.ArtifactVersion, error) {
	return artifactversion.ArtifactVersion{}, errors.New("not implemented")
}
func (*artifactVersionMemory) GetBySemanticVersion(context.Context, shared.UUID, shared.UUID, artifactversion.SemanticVersion) (artifactversion.ArtifactVersion, error) {
	return artifactversion.ArtifactVersion{}, errors.New("not implemented")
}
func (*artifactVersionMemory) List(context.Context, shared.UUID, shared.UUID, *shared.UUID, int) ([]artifactversion.ArtifactVersion, error) {
	return nil, errors.New("not implemented")
}

type artifactSourceMemory struct {
	values         []artifactsource.ArtifactSource
	creates        int
	auditPrincipal string
}

func (memory *artifactSourceMemory) Create(ctx context.Context, value artifactsource.ArtifactSource) error {
	actor, ok := audit.ActorFromContext(ctx)
	if !ok {
		return errors.New("audit actor missing")
	}
	memory.auditPrincipal = actor.PrincipalID()
	memory.values = append(memory.values, value)
	memory.creates++
	return nil
}
func (memory *artifactSourceMemory) Get(_ context.Context, tenantID, versionID shared.UUID) (artifactsource.ArtifactSource, error) {
	for _, value := range memory.values {
		if value.TenantID() == tenantID && value.ArtifactVersionID() == versionID {
			return value, nil
		}
	}
	return artifactsource.ArtifactSource{}, typed(shared.ErrorNotFound, "artifact_source.not_found")
}
