package localauth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const tenantID = "0198fc21-ced5-7000-8000-000000000001"

func TestAuthenticatorReturnsVisiblyLocalFixedIdentity(t *testing.T) {
	authenticator, err := New(config.ModeDevelopment, localConfig("alice"))
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := authenticator.Authenticate(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	wantTenant, _ := shared.ParseUUID(tenantID)
	if mapped.TenantID != wantTenant || mapped.PrincipalID != "local-development:alice" {
		t.Fatalf("unexpected identity: %#v", mapped)
	}
	if !strings.HasPrefix(mapped.PrincipalID, principalPrefix) {
		t.Fatalf("principal is not visibly local: %q", mapped.PrincipalID)
	}
}

func TestAuthenticatorRejectsBearerCredentials(t *testing.T) {
	authenticator, err := New(config.ModeDevelopment, localConfig("alice"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = authenticator.Authenticate(context.Background(), "caller-controlled")
	var typed *shared.TypedError
	if !errors.As(err, &typed) || typed.Class() != shared.ErrorUnauthorized || typed.Code().String() != "identity.local_credential_forbidden" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRejectsInvalidOrUnsafeMode(t *testing.T) {
	invalidTenant := localConfig("alice")
	invalidTenant.LocalDevelopment.TenantID = "not-a-uuid"
	for name, test := range map[string]struct {
		mode       config.Mode
		configured config.AuthenticationConfig
	}{
		"production": {mode: config.ModeProduction, configured: localConfig("alice")},
		"test":       {mode: config.ModeTest, configured: localConfig("alice")},
		"OIDC mode":  {mode: config.ModeDevelopment, configured: config.AuthenticationConfig{Mode: config.AuthenticationModeOIDC}},
		"tenant":     {mode: config.ModeDevelopment, configured: invalidTenant},
		"principal":  {mode: config.ModeDevelopment, configured: localConfig("bad\nprincipal")},
		"oversized":  {mode: config.ModeDevelopment, configured: localConfig(strings.Repeat("x", 238))},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(test.mode, test.configured); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func localConfig(principal string) config.AuthenticationConfig {
	return config.AuthenticationConfig{Mode: config.AuthenticationModeLocalDevelopment,
		LocalDevelopment: config.LocalDevelopmentAuthConfig{TenantID: tenantID, Principal: principal}}
}
