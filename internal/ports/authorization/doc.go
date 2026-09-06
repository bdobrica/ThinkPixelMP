// Package authorization defines marketplace administrative authorization
// without exposing a transport, identity provider, or policy implementation to
// application code.
package authorization

import (
	"context"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

// Role is one explicitly granted, tenant-scoped marketplace administrative
// role. Roles have no implied inheritance.
type Role string

const (
	RolePublisherAdmin        Role = "publisher-admin"
	RoleNamespaceAdmin        Role = "namespace-admin"
	RolePublicationAdmin      Role = "publication-admin"
	RoleEvidenceProducerAdmin Role = "evidence-producer-admin"
	RoleReviewer              Role = "reviewer"
	RoleCatalogAdmin          Role = "catalog-admin"
	RolePolicyAdmin           Role = "policy-admin"
	RoleRevocationAdmin       Role = "revocation-admin"
	RoleFederationAdmin       Role = "federation-admin"
)

// Action identifies an administrative authorization boundary. Authorization
// does not replace resource-specific domain checks performed after it.
type Action string

const (
	ActionManagePublishers        Action = "publisher.manage"
	ActionManageNamespaces        Action = "namespace.manage"
	ActionPublish                 Action = "publication.publish"
	ActionManageEvidenceProducers Action = "evidence_producer.manage"
	ActionReviewPromotion         Action = "promotion.review"
	ActionManageCatalogs          Action = "catalog.manage"
	ActionActivatePolicy          Action = "policy.activate"
	ActionManageRevocations       Action = "revocation.manage"
	ActionManageFederation        Action = "federation.manage"
)

// Grant explicitly assigns one role to one principal in one tenant.
type Grant struct {
	TenantID    shared.UUID
	PrincipalID string
	Role        Role
}

// Authorizer checks administrative authority established outside untrusted
// request, artifact, evidence, federation, and policy-output data.
type Authorizer interface {
	Authorize(context.Context, identity.Identity, Action) error
}
