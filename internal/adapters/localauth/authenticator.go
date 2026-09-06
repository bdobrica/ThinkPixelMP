// Package localauth supplies the explicitly configured, fixed-identity
// authenticator used only by disposable local development processes.
package localauth

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

const principalPrefix = "local-development:"

var errUnexpectedCredential = mustIdentityError()

// Authenticator always returns one operator-configured identity. It never
// reads identity or authority from request data.
type Authenticator struct {
	identity identity.Identity
}

func New(processMode config.Mode, configured config.AuthenticationConfig) (*Authenticator, error) {
	if processMode != config.ModeDevelopment || configured.Mode != config.AuthenticationModeLocalDevelopment {
		return nil, errors.New("local authentication: requires explicit local-development authentication in development process mode")
	}
	tenantID, err := shared.ParseUUID(configured.LocalDevelopment.TenantID)
	if err != nil {
		return nil, errors.New("local authentication: tenant ID must be a UUIDv7")
	}
	principal := configured.LocalDevelopment.Principal
	if len(principal) == 0 || len(principal) > 237 || !utf8.ValidString(principal) ||
		strings.IndexFunc(principal, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return nil, errors.New("local authentication: principal must contain 1 to 237 valid UTF-8 bytes without control characters")
	}
	return &Authenticator{identity: identity.Identity{TenantID: tenantID, PrincipalID: principalPrefix + principal}}, nil
}

// Authenticate accepts no bearer credential. Supplying one fails closed so a
// development request cannot appear to have verified that credential.
func (authenticator *Authenticator) Authenticate(_ context.Context, credential string) (identity.Identity, error) {
	if credential != "" {
		return identity.Identity{}, errUnexpectedCredential
	}
	return authenticator.identity, nil
}

func mustIdentityError() error {
	code, err := shared.NewReasonCode("identity.local_credential_forbidden")
	if err != nil {
		panic(err)
	}
	return shared.NewTypedError(shared.ErrorUnauthorized, code)
}

var _ identity.Authenticator = (*Authenticator)(nil)
