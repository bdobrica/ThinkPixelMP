package security

import (
	"context"
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

const maximumPrincipalIDBytes = 255

var (
	errIdentityRequired    = mustAuthorizationError(shared.ErrorUnauthorized, "authorization.identity_required")
	errAuthorizationDenied = mustAuthorizationError(shared.ErrorForbidden, "authorization.denied")
)

var actionsByRole = map[authorization.Role]map[authorization.Action]struct{}{
	authorization.RolePublisherAdmin:        {authorization.ActionManagePublishers: {}},
	authorization.RoleNamespaceAdmin:        {authorization.ActionManageNamespaces: {}},
	authorization.RolePublicationAdmin:      {authorization.ActionPublish: {}},
	authorization.RoleEvidenceProducerAdmin: {authorization.ActionManageEvidenceProducers: {}},
	authorization.RoleReviewer:              {authorization.ActionReviewPromotion: {}},
	authorization.RoleCatalogAdmin:          {authorization.ActionManageCatalogs: {}},
	authorization.RolePolicyAdmin:           {authorization.ActionActivatePolicy: {}},
	authorization.RoleRevocationAdmin:       {authorization.ActionManageRevocations: {}},
	authorization.RoleFederationAdmin:       {authorization.ActionManageFederation: {}},
}

type principalKey struct {
	tenantID    string
	principalID string
}

// Authorizer is an immutable, fail-closed marketplace administrative role
// policy. Grant provisioning is an operator-owned bootstrap concern; callers
// cannot add authority through request or marketplace content.
type Authorizer struct {
	rolesByPrincipal map[principalKey]map[authorization.Role]struct{}
}

// NewAuthorizer validates and copies explicit tenant/principal role grants.
func NewAuthorizer(grants []authorization.Grant) (*Authorizer, error) {
	roles := make(map[principalKey]map[authorization.Role]struct{})
	for index, grant := range grants {
		if _, err := grant.TenantID.MarshalText(); err != nil {
			return nil, fmt.Errorf("authorization grant %d: invalid tenant ID", index)
		}
		if err := validatePrincipalID(grant.PrincipalID); err != nil {
			return nil, fmt.Errorf("authorization grant %d: principal ID: %w", index, err)
		}
		if _, known := actionsByRole[grant.Role]; !known {
			return nil, fmt.Errorf("authorization grant %d: unknown role %q", index, grant.Role)
		}
		key := principalKey{tenantID: grant.TenantID.String(), principalID: grant.PrincipalID}
		if roles[key] == nil {
			roles[key] = make(map[authorization.Role]struct{})
		}
		if _, duplicate := roles[key][grant.Role]; duplicate {
			return nil, fmt.Errorf("authorization grant %d: duplicate role grant", index)
		}
		roles[key][grant.Role] = struct{}{}
	}
	return &Authorizer{rolesByPrincipal: roles}, nil
}

// Authorize permits only an action attached to an explicitly granted role in
// the mapped identity's exact tenant. Unknown actions fail closed.
func (authorizer *Authorizer) Authorize(_ context.Context, mapped identity.Identity, action authorization.Action) error {
	if _, err := mapped.TenantID.MarshalText(); err != nil || validatePrincipalID(mapped.PrincipalID) != nil {
		return errIdentityRequired
	}
	roles := authorizer.rolesByPrincipal[principalKey{tenantID: mapped.TenantID.String(), principalID: mapped.PrincipalID}]
	for role := range roles {
		if _, allowed := actionsByRole[role][action]; allowed {
			return nil
		}
	}
	return errAuthorizationDenied
}

func validatePrincipalID(value string) error {
	if value == "" || len(value) > maximumPrincipalIDBytes || !utf8.ValidString(value) {
		return errors.New("must contain 1 to 255 valid UTF-8 bytes")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("must not contain control characters")
		}
	}
	return nil
}

func mustAuthorizationError(class shared.ErrorClass, code string) error {
	reason, err := shared.NewReasonCode(code)
	if err != nil {
		panic(err)
	}
	return shared.NewTypedError(class, reason)
}

var _ authorization.Authorizer = (*Authorizer)(nil)
