// Package identity defines authenticated identity operations without exposing a
// particular OIDC or JWT implementation to application code.
package identity

import (
	"context"
	"encoding/json"
	"time"
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
