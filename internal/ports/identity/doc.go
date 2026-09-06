// Package identity defines authenticated identity operations without exposing a
// particular OIDC or JWT implementation to application code.
package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

// VerifiedToken is the cryptographically verified identity assertion consumed
// by the separately configured tenant and principal mapper.
type VerifiedToken struct {
	Issuer    string
	Subject   string
	Audiences []string
	ExpiresAt time.Time
	NotBefore *time.Time
	Claims    json.RawMessage
}

// Verifier verifies an encoded bearer token and its standard claims. It does
// not assign a tenant, principal, role, or administrative authority.
type Verifier interface {
	Verify(context.Context, string) (VerifiedToken, error)
}

// Identity is the tenant-scoped principal established from a verified token
// through operator-controlled mapping configuration. PrincipalID is opaque and
// safe to use as the actor and idempotency owner identifier.
type Identity struct {
	TenantID    shared.UUID
	PrincipalID string
}

// Authenticator establishes an identity from transport-extracted bearer
// material. An empty credential is meaningful only to an explicitly selected
// local-development adapter; production authenticators fail closed.
type Authenticator interface {
	Authenticate(context.Context, string) (Identity, error)
}

// Mapper assigns marketplace tenant and principal identity from verified
// claims. It does not grant roles or administrative authority.
type Mapper interface {
	Map(VerifiedToken) (Identity, error)
}
