// Package oidc verifies JWT identity assertions from configured OpenID Connect
// issuers. Claim-to-tenant/principal mapping remains behind the identity port.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

const maximumTokenBytes = 64 << 10

var errInvalidToken = mustTypedError(shared.ErrorUnauthorized, "identity.invalid_token")

// Config contains the trust-bearing values for exactly one issuer.
type Config struct {
	Issuer            string
	Audience          string
	AllowedAlgorithms []string
	ClockSkew         time.Duration
	DiscoveryTimeout  time.Duration
}

// Verifier validates signatures and standard OIDC claims without assigning
// marketplace authority.
type Verifier struct {
	upstream *coreoidc.IDTokenVerifier
	clock    clock.Clock
	skew     time.Duration
	audience string
	issuer   string
}

// Discover validates provider metadata and creates a verifier backed by the
// issuer's rotating remote JWKS. Discovery is bounded by configuration.
func Discover(ctx context.Context, configured Config, current clock.Clock) (*Verifier, error) {
	if err := validateConfig(configured, current); err != nil {
		return nil, err
	}
	discoveryContext, cancel := context.WithTimeout(ctx, configured.DiscoveryTimeout)
	defer cancel()
	provider, err := coreoidc.NewProvider(discoveryContext, configured.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return newVerifier(configured, current, provider.Verifier(verifierConfig(configured, current))), nil
}

// New creates a verifier from a supplied key set. It supports offline issuer
// configuration and deterministic tests while retaining identical validation.
func New(configured Config, keys coreoidc.KeySet, current clock.Clock) (*Verifier, error) {
	if err := validateConfig(configured, current); err != nil {
		return nil, err
	}
	if keys == nil {
		return nil, errors.New("oidc verifier: key set is required")
	}
	upstream := coreoidc.NewVerifier(configured.Issuer, keys, verifierConfig(configured, current))
	return newVerifier(configured, current, upstream), nil
}

func newVerifier(configured Config, current clock.Clock, upstream *coreoidc.IDTokenVerifier) *Verifier {
	return &Verifier{upstream: upstream, clock: current, skew: configured.ClockSkew, audience: configured.Audience, issuer: configured.Issuer}
}

func verifierConfig(configured Config, current clock.Clock) *coreoidc.Config {
	return &coreoidc.Config{
		ClientID:             configured.Audience,
		SupportedSigningAlgs: append([]string(nil), configured.AllowedAlgorithms...),
		// Expiry and not-before are checked below with the configured bounded skew.
		SkipExpiryCheck: true,
		Now:             current.Now,
	}
}

func validateConfig(configured Config, current clock.Clock) error {
	if current == nil {
		return errors.New("oidc verifier: clock is required")
	}
	u, err := url.Parse(configured.Issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("oidc verifier: issuer must be an HTTPS URL without user information, query, or fragment")
	}
	if configured.Audience == "" {
		return errors.New("oidc verifier: audience is required")
	}
	if len(configured.AllowedAlgorithms) == 0 {
		return errors.New("oidc verifier: allowed algorithms are required")
	}
	seen := make(map[string]struct{}, len(configured.AllowedAlgorithms))
	for _, algorithm := range configured.AllowedAlgorithms {
		switch algorithm {
		case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA":
		default:
			return fmt.Errorf("oidc verifier: unsupported algorithm %q", algorithm)
		}
		if _, exists := seen[algorithm]; exists {
			return fmt.Errorf("oidc verifier: duplicate algorithm %q", algorithm)
		}
		seen[algorithm] = struct{}{}
	}
	if configured.ClockSkew < 0 || configured.ClockSkew > 5*time.Minute {
		return errors.New("oidc verifier: clock skew must be between 0 and 5m")
	}
	if configured.DiscoveryTimeout <= 0 || configured.DiscoveryTimeout > time.Minute {
		return errors.New("oidc verifier: discovery timeout must be positive and at most 1m")
	}
	return nil
}

// Verify authenticates a compact JWT and returns only verified claims. All
// token failures intentionally collapse to one stable unauthorized reason.
func (verifier *Verifier) Verify(ctx context.Context, raw string) (identity.VerifiedToken, error) {
	if len(raw) == 0 || len(raw) > maximumTokenBytes || strings.Count(raw, ".") != 2 {
		return identity.VerifiedToken{}, errInvalidToken
	}
	token, err := verifier.upstream.Verify(ctx, raw)
	if err != nil {
		return identity.VerifiedToken{}, errInvalidToken
	}
	// Enforce literal equality even for issuers for which the upstream library
	// retains a compatibility exception.
	if token.Issuer != verifier.issuer {
		return identity.VerifiedToken{}, errInvalidToken
	}
	var rawClaims json.RawMessage
	if err := token.Claims(&rawClaims); err != nil || len(rawClaims) == 0 {
		return identity.VerifiedToken{}, errInvalidToken
	}
	var claims registeredClaims
	decoder := json.NewDecoder(strings.NewReader(string(rawClaims)))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil || claims.Subject == "" || claims.Expiry == "" {
		return identity.VerifiedToken{}, errInvalidToken
	}
	if (len(token.Audience) > 1 || claims.AuthorizedParty != "") && claims.AuthorizedParty != verifier.audience {
		return identity.VerifiedToken{}, errInvalidToken
	}
	expiresAt, ok := numericDate(claims.Expiry)
	if !ok {
		return identity.VerifiedToken{}, errInvalidToken
	}
	now := verifier.clock.Now()
	if !now.Before(expiresAt.Add(verifier.skew)) {
		return identity.VerifiedToken{}, errInvalidToken
	}
	var notBefore *time.Time
	if claims.NotBefore != "" {
		parsed, valid := numericDate(claims.NotBefore)
		if !valid || now.Add(verifier.skew).Before(parsed) {
			return identity.VerifiedToken{}, errInvalidToken
		}
		notBefore = &parsed
	}
	return identity.VerifiedToken{
		Issuer: token.Issuer, Subject: claims.Subject,
		Audiences: append([]string(nil), token.Audience...),
		ExpiresAt: expiresAt, NotBefore: notBefore,
		Claims: append(json.RawMessage(nil), rawClaims...),
	}, nil
}

type registeredClaims struct {
	Subject         string      `json:"sub"`
	Expiry          json.Number `json:"exp"`
	NotBefore       json.Number `json:"nbf"`
	AuthorizedParty string      `json:"azp"`
}

func numericDate(number json.Number) (time.Time, bool) {
	seconds, err := number.Int64()
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0).UTC(), true
}

func mustTypedError(class shared.ErrorClass, code string) error {
	reason, err := shared.NewReasonCode(code)
	if err != nil {
		panic(err)
	}
	return shared.NewTypedError(class, reason)
}

var _ identity.Verifier = (*Verifier)(nil)
