package oidc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

const (
	tenantA = "0198fc21-ced5-7000-8000-000000000001"
	tenantB = "0198fc21-ced5-7000-8000-000000000002"
)

func TestVerifiedJWTMapsThroughConfiguredPolicy(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key := generateKey(t)
	verifier := newTestVerifier(t, key, now, 30*time.Second)
	raw := signToken(t, key, "RS256", map[string]any{
		"iss": "https://issuer.example.test", "sub": "person-17", "aud": "thinkpixelmp",
		"exp": now.Add(time.Minute).Unix(), "groups": []string{"marketplace-a"},
	})
	verified, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := NewMapper(MappingConfig{Issuer: "https://issuer.example.test", TenantClaim: "groups", PrincipalClaim: "sub",
		TenantMappings: []TenantMapping{{ClaimValue: "marketplace-a", TenantID: tenantA}}})
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := mapper.Map(verified)
	if err != nil {
		t.Fatal(err)
	}
	wantTenant, _ := shared.ParseUUID(tenantA)
	if mapped.TenantID != wantTenant || mapped.PrincipalID != opaquePrincipalID(verified.Issuer, verified.Subject) {
		t.Fatalf("unexpected mapped identity: %#v", mapped)
	}
}

func TestMapperDerivesConfiguredTenantAndOpaquePrincipal(t *testing.T) {
	mapper := newTestMapper(t)
	token := verifiedToken(t, map[string]any{"groups": []string{"unrelated", "marketplace-a"}, "employee_id": "person-17"})

	mapped, err := mapper.Map(token)
	if err != nil {
		t.Fatal(err)
	}
	wantTenant, _ := shared.ParseUUID(tenantA)
	if mapped.TenantID != wantTenant {
		t.Fatalf("tenant = %s, want %s", mapped.TenantID, wantTenant)
	}
	if mapped.PrincipalID != opaquePrincipalID(token.Issuer, "person-17") {
		t.Fatalf("unexpected principal ID %q", mapped.PrincipalID)
	}
	if strings.Contains(mapped.PrincipalID, "person-17") || len(mapped.PrincipalID) > 255 {
		t.Fatalf("principal ID exposes raw claim or exceeds persistence bound: %q", mapped.PrincipalID)
	}

	again, err := mapper.Map(token)
	if err != nil || again != mapped {
		t.Fatalf("mapping is not deterministic: %#v, %v", again, err)
	}
}

func TestMapperAcceptsScalarTenantClaim(t *testing.T) {
	mapper := newTestMapper(t)
	mapped, err := mapper.Map(verifiedToken(t, map[string]any{"groups": "marketplace-b", "employee_id": "person-17"}))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := shared.ParseUUID(tenantB)
	if mapped.TenantID != want {
		t.Fatalf("tenant = %s, want %s", mapped.TenantID, want)
	}
}

func TestMapperFailsClosedForUnmappedOrAmbiguousClaims(t *testing.T) {
	mapper := newTestMapper(t)
	tests := map[string]identity.VerifiedToken{
		"wrong issuer":      verifiedTokenForIssuer(t, "https://other.example.test", map[string]any{"groups": "marketplace-a", "employee_id": "p"}),
		"missing tenant":    verifiedToken(t, map[string]any{"employee_id": "p"}),
		"unmapped tenant":   verifiedToken(t, map[string]any{"groups": "unknown", "employee_id": "p"}),
		"ambiguous tenant":  verifiedToken(t, map[string]any{"groups": []string{"marketplace-a", "marketplace-b"}, "employee_id": "p"}),
		"missing principal": verifiedToken(t, map[string]any{"groups": "marketplace-a"}),
		"principal array":   verifiedToken(t, map[string]any{"groups": "marketplace-a", "employee_id": []string{"p"}}),
		"non-string tenant": verifiedToken(t, map[string]any{"groups": 7, "employee_id": "p"}),
		"empty principal":   verifiedToken(t, map[string]any{"groups": "marketplace-a", "employee_id": ""}),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := mapper.Map(token)
			assertTypedError(t, err, shared.ErrorUnauthorized, "identity.unmapped")
		})
	}
}

func TestMapperRejectsDuplicateClaimNames(t *testing.T) {
	mapper := newTestMapper(t)
	token := identity.VerifiedToken{Issuer: "https://issuer.example.test", Claims: json.RawMessage(`{"groups":"marketplace-a","groups":"marketplace-b","employee_id":"p"}`)}
	_, err := mapper.Map(token)
	assertTypedError(t, err, shared.ErrorUnauthorized, "identity.unmapped")
}

func TestMapperRequiresVerifiedSubjectWhenPrincipalClaimIsSub(t *testing.T) {
	mapper, err := NewMapper(MappingConfig{Issuer: "https://issuer.example.test", TenantClaim: "groups", PrincipalClaim: "sub",
		TenantMappings: []TenantMapping{{ClaimValue: "marketplace-a", TenantID: tenantA}}})
	if err != nil {
		t.Fatal(err)
	}
	token := verifiedToken(t, map[string]any{"groups": "marketplace-a", "sub": "different-subject"})
	_, err = mapper.Map(token)
	assertTypedError(t, err, shared.ErrorUnauthorized, "identity.unmapped")
}

func TestNewMapperRejectsInvalidPolicy(t *testing.T) {
	valid := func() MappingConfig {
		return MappingConfig{Issuer: "https://issuer.example.test", TenantClaim: "groups", PrincipalClaim: "employee_id",
			TenantMappings: []TenantMapping{{ClaimValue: "marketplace-a", TenantID: tenantA}}}
	}
	for name, mutate := range map[string]func(*MappingConfig){
		"missing issuer":   func(c *MappingConfig) { c.Issuer = "" },
		"missing claim":    func(c *MappingConfig) { c.TenantClaim = "" },
		"control in claim": func(c *MappingConfig) { c.PrincipalClaim = "employee\nidentity" },
		"no mappings":      func(c *MappingConfig) { c.TenantMappings = nil },
		"invalid tenant":   func(c *MappingConfig) { c.TenantMappings[0].TenantID = "not-a-uuid" },
		"duplicate value": func(c *MappingConfig) {
			c.TenantMappings = append(c.TenantMappings, c.TenantMappings[0])
		},
	} {
		t.Run(name, func(t *testing.T) {
			configured := valid()
			mutate(&configured)
			if _, err := NewMapper(configured); err == nil {
				t.Fatal("expected mapping policy error")
			}
		})
	}
}

func newTestMapper(t *testing.T) *Mapper {
	t.Helper()
	mapper, err := NewMapper(MappingConfig{Issuer: "https://issuer.example.test", TenantClaim: "groups", PrincipalClaim: "employee_id",
		TenantMappings: []TenantMapping{{ClaimValue: "marketplace-a", TenantID: tenantA}, {ClaimValue: "marketplace-b", TenantID: tenantB}}})
	if err != nil {
		t.Fatal(err)
	}
	return mapper
}

func verifiedToken(t *testing.T, claims map[string]any) identity.VerifiedToken {
	t.Helper()
	return verifiedTokenForIssuer(t, "https://issuer.example.test", claims)
}

func verifiedTokenForIssuer(t *testing.T, issuer string, claims map[string]any) identity.VerifiedToken {
	t.Helper()
	raw, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return identity.VerifiedToken{Issuer: issuer, Subject: "verified-subject", ExpiresAt: time.Now().Add(time.Minute), Claims: raw}
}
