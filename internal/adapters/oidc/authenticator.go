package oidc

import (
	"context"
	"errors"

	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

// Authenticator composes cryptographic verification with operator-configured
// tenant/principal mapping for an HTTP bearer credential.
type Authenticator struct {
	verifier identity.Verifier
	mapper   identity.Mapper
}

func NewAuthenticator(verifier identity.Verifier, mapper identity.Mapper) (*Authenticator, error) {
	if verifier == nil || mapper == nil {
		return nil, errors.New("oidc authenticator: verifier and mapper are required")
	}
	return &Authenticator{verifier: verifier, mapper: mapper}, nil
}

func (authenticator *Authenticator) Authenticate(ctx context.Context, credential string) (identity.Identity, error) {
	verified, err := authenticator.verifier.Verify(ctx, credential)
	if err != nil {
		return identity.Identity{}, err
	}
	return authenticator.mapper.Map(verified)
}

var _ identity.Authenticator = (*Authenticator)(nil)
