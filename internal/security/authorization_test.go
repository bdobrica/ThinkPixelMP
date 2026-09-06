package security

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/authorization"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

func TestAdministrativeRolesAuthorizeOnlyTheirActions(t *testing.T) {
	tenant := authorizationID(t, "0198fc21-ced5-7000-8000-000000000001")
	cases := []struct {
		role   authorization.Role
		action authorization.Action
	}{
		{authorization.RolePublisherAdmin, authorization.ActionManagePublishers},
		{authorization.RoleNamespaceAdmin, authorization.ActionManageNamespaces},
		{authorization.RolePublicationAdmin, authorization.ActionPublish},
		{authorization.RoleEvidenceProducerAdmin, authorization.ActionManageEvidenceProducers},
		{authorization.RoleReviewer, authorization.ActionReviewPromotion},
		{authorization.RoleCatalogAdmin, authorization.ActionManageCatalogs},
		{authorization.RolePolicyAdmin, authorization.ActionActivatePolicy},
		{authorization.RoleRevocationAdmin, authorization.ActionManageRevocations},
		{authorization.RoleFederationAdmin, authorization.ActionManageFederation},
	}
	for _, tc := range cases {
		t.Run(string(tc.role), func(t *testing.T) {
			authorizer, err := NewAuthorizer([]authorization.Grant{{TenantID: tenant, PrincipalID: "oidc:principal", Role: tc.role}})
			if err != nil {
				t.Fatal(err)
			}
			mapped := identity.Identity{TenantID: tenant, PrincipalID: "oidc:principal"}
			if err := authorizer.Authorize(context.Background(), mapped, tc.action); err != nil {
				t.Fatalf("expected %s to allow %s: %v", tc.role, tc.action, err)
			}
			for _, other := range cases {
				if other.action == tc.action {
					continue
				}
				assertAuthorizationError(t, authorizer.Authorize(context.Background(), mapped, other.action), shared.ErrorForbidden, "authorization.denied")
			}
		})
	}
}

func TestAdministrativeGrantsComposeAndRemainTenantScoped(t *testing.T) {
	tenantA := authorizationID(t, "0198fc21-ced5-7000-8000-000000000001")
	tenantB := authorizationID(t, "0198fc21-ced5-7000-8000-000000000002")
	authorizer, err := NewAuthorizer([]authorization.Grant{
		{TenantID: tenantA, PrincipalID: "oidc:principal", Role: authorization.RoleCatalogAdmin},
		{TenantID: tenantA, PrincipalID: "oidc:principal", Role: authorization.RoleReviewer},
		{TenantID: tenantB, PrincipalID: "oidc:principal", Role: authorization.RoleFederationAdmin},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []authorization.Action{authorization.ActionManageCatalogs, authorization.ActionReviewPromotion} {
		if err := authorizer.Authorize(context.Background(), identity.Identity{TenantID: tenantA, PrincipalID: "oidc:principal"}, action); err != nil {
			t.Fatalf("composed grant denied %s: %v", action, err)
		}
	}
	assertAuthorizationError(t, authorizer.Authorize(context.Background(), identity.Identity{TenantID: tenantA, PrincipalID: "oidc:principal"}, authorization.ActionManageFederation), shared.ErrorForbidden, "authorization.denied")
	assertAuthorizationError(t, authorizer.Authorize(context.Background(), identity.Identity{TenantID: tenantB, PrincipalID: "oidc:principal"}, authorization.ActionManageCatalogs), shared.ErrorForbidden, "authorization.denied")
	assertAuthorizationError(t, authorizer.Authorize(context.Background(), identity.Identity{TenantID: tenantA, PrincipalID: "oidc:other"}, authorization.ActionManageCatalogs), shared.ErrorForbidden, "authorization.denied")
}

func TestAdministrativeAuthorizationFailsClosed(t *testing.T) {
	tenant := authorizationID(t, "0198fc21-ced5-7000-8000-000000000001")
	authorizer, err := NewAuthorizer(nil)
	if err != nil {
		t.Fatal(err)
	}
	assertAuthorizationError(t, authorizer.Authorize(context.Background(), identity.Identity{}, authorization.ActionManageCatalogs), shared.ErrorUnauthorized, "authorization.identity_required")
	assertAuthorizationError(t, authorizer.Authorize(context.Background(), identity.Identity{TenantID: tenant, PrincipalID: "principal"}, authorization.Action("catalog.delete_everything")), shared.ErrorForbidden, "authorization.denied")
}

func TestAdministrativeGrantValidation(t *testing.T) {
	tenant := authorizationID(t, "0198fc21-ced5-7000-8000-000000000001")
	valid := authorization.Grant{TenantID: tenant, PrincipalID: "oidc:principal", Role: authorization.RoleCatalogAdmin}
	for name, grants := range map[string][]authorization.Grant{
		"invalid tenant":    {{PrincipalID: valid.PrincipalID, Role: valid.Role}},
		"empty principal":   {{TenantID: tenant, Role: valid.Role}},
		"control principal": {{TenantID: tenant, PrincipalID: "principal\nadmin", Role: valid.Role}},
		"long principal":    {{TenantID: tenant, PrincipalID: strings.Repeat("x", 256), Role: valid.Role}},
		"unknown role":      {{TenantID: tenant, PrincipalID: valid.PrincipalID, Role: "marketplace-admin"}},
		"duplicate role":    {valid, valid},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAuthorizer(grants); err == nil {
				t.Fatal("expected grant validation error")
			}
		})
	}
}

func authorizationID(t *testing.T, value string) shared.UUID {
	t.Helper()
	id, err := shared.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertAuthorizationError(t *testing.T, err error, class shared.ErrorClass, code string) {
	t.Helper()
	var typed *shared.TypedError
	if !errors.As(err, &typed) || typed.Class() != class || typed.Code().String() != code {
		t.Fatalf("unexpected error: %v", err)
	}
}
