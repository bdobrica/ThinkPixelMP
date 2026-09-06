package oidc

import (
	"context"
	"errors"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

func TestAuthenticatorComposesVerificationAndMapping(t *testing.T) {
	tenant, err := shared.ParseUUID("01991e80-2c00-7000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	verifier := &verifierFake{token: identity.VerifiedToken{Issuer: "https://issuer.example", Subject: "subject"}}
	mapper := &mapperFake{identity: identity.Identity{TenantID: tenant, PrincipalID: "oidc:opaque"}}
	authenticator, err := NewAuthenticator(verifier, mapper)
	if err != nil {
		t.Fatal(err)
	}
	got, err := authenticator.Authenticate(context.Background(), "signed-token")
	if err != nil || got != mapper.identity || verifier.credential != "signed-token" || mapper.token.Subject != "subject" {
		t.Fatalf("authentication: %#v %v", got, err)
	}

	verificationFailure := errors.New("verification failed")
	verifier.err = verificationFailure
	if _, err := authenticator.Authenticate(context.Background(), "bad"); !errors.Is(err, verificationFailure) {
		t.Fatalf("verification failure changed: %v", err)
	}
}

type verifierFake struct {
	token      identity.VerifiedToken
	credential string
	err        error
}

func (fake *verifierFake) Verify(_ context.Context, credential string) (identity.VerifiedToken, error) {
	fake.credential = credential
	return fake.token, fake.err
}

type mapperFake struct {
	identity identity.Identity
	token    identity.VerifiedToken
}

func (fake *mapperFake) Map(token identity.VerifiedToken) (identity.Identity, error) {
	fake.token = token
	return fake.identity, nil
}
