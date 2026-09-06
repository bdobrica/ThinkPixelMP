package oidc

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

const (
	maximumClaimNameBytes  = 128
	maximumClaimValueBytes = 1024
	maximumTenantMappings  = 10000
)

var errUnmappedIdentity = mustTypedError(shared.ErrorUnauthorized, "identity.unmapped")

// TenantMapping binds exactly one verified tenant-claim value to one local
// UUIDv7 tenant. Claim values never become tenant authority directly.
type TenantMapping struct {
	ClaimValue string
	TenantID   string
}

// MappingConfig contains the operator-owned mapping policy for one issuer.
// Claim names address top-level JWT claims only.
type MappingConfig struct {
	Issuer         string
	TenantClaim    string
	PrincipalClaim string
	TenantMappings []TenantMapping
}

// Mapper maps verified claims to a local tenant and opaque principal ID.
type Mapper struct {
	issuer         string
	tenantClaim    string
	principalClaim string
	tenantByValue  map[string]shared.UUID
}

// NewMapper validates and copies an issuer-specific mapping policy.
func NewMapper(configured MappingConfig) (*Mapper, error) {
	issuerURL, err := url.Parse(configured.Issuer)
	if err != nil || issuerURL.Scheme != "https" || issuerURL.Host == "" || issuerURL.User != nil || issuerURL.RawQuery != "" || issuerURL.Fragment != "" {
		return nil, errors.New("oidc mapper: issuer must be an HTTPS URL without user information, query, or fragment")
	}
	if err := validateClaimName(configured.TenantClaim); err != nil {
		return nil, fmt.Errorf("oidc mapper: tenant claim: %w", err)
	}
	if err := validateClaimName(configured.PrincipalClaim); err != nil {
		return nil, fmt.Errorf("oidc mapper: principal claim: %w", err)
	}
	if len(configured.TenantMappings) == 0 || len(configured.TenantMappings) > maximumTenantMappings {
		return nil, fmt.Errorf("oidc mapper: tenant mappings must contain 1 to %d entries", maximumTenantMappings)
	}
	mappings := make(map[string]shared.UUID, len(configured.TenantMappings))
	for index, mapping := range configured.TenantMappings {
		if err := validateClaimValue(mapping.ClaimValue); err != nil {
			return nil, fmt.Errorf("oidc mapper: tenant mapping %d claim value: %w", index, err)
		}
		if _, exists := mappings[mapping.ClaimValue]; exists {
			return nil, fmt.Errorf("oidc mapper: duplicate tenant claim value at mapping %d", index)
		}
		tenantID, err := shared.ParseUUID(mapping.TenantID)
		if err != nil {
			return nil, fmt.Errorf("oidc mapper: tenant mapping %d tenant ID: %w", index, err)
		}
		mappings[mapping.ClaimValue] = tenantID
	}
	return &Mapper{issuer: configured.Issuer, tenantClaim: configured.TenantClaim,
		principalClaim: configured.PrincipalClaim, tenantByValue: mappings}, nil
}

// Map fails closed when claims are absent, malformed, unmapped, or match more
// than one configured tenant mapping. All runtime mapping failures intentionally
// share one bounded unauthorized classification.
func (mapper *Mapper) Map(token identity.VerifiedToken) (identity.Identity, error) {
	if token.Issuer != mapper.issuer || len(token.Claims) == 0 {
		return identity.Identity{}, errUnmappedIdentity
	}
	claims, err := decodeUniqueClaims(token.Claims)
	if err != nil {
		return identity.Identity{}, errUnmappedIdentity
	}
	tenantValues, ok := stringClaimValues(claims[mapper.tenantClaim], true)
	if !ok {
		return identity.Identity{}, errUnmappedIdentity
	}
	var tenantID shared.UUID
	matches := 0
	seenValues := make(map[string]struct{}, len(tenantValues))
	for _, value := range tenantValues {
		if _, duplicate := seenValues[value]; duplicate {
			continue
		}
		seenValues[value] = struct{}{}
		if mapped, exists := mapper.tenantByValue[value]; exists {
			tenantID = mapped
			matches++
		}
	}
	if matches != 1 {
		return identity.Identity{}, errUnmappedIdentity
	}
	principalValues, ok := stringClaimValues(claims[mapper.principalClaim], false)
	if !ok || len(principalValues) != 1 || validateClaimValue(principalValues[0]) != nil {
		return identity.Identity{}, errUnmappedIdentity
	}
	if mapper.principalClaim == "sub" && principalValues[0] != token.Subject {
		return identity.Identity{}, errUnmappedIdentity
	}
	return identity.Identity{TenantID: tenantID, PrincipalID: opaquePrincipalID(mapper.issuer, principalValues[0])}, nil
}

func validateClaimName(value string) error {
	if len(value) == 0 || len(value) > maximumClaimNameBytes || !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("must contain 1 to %d valid UTF-8 bytes without control characters", maximumClaimNameBytes)
	}
	return nil
}

func validateClaimValue(value string) error {
	if len(value) == 0 || len(value) > maximumClaimValueBytes || !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("must contain 1 to %d valid UTF-8 bytes without control characters", maximumClaimValueBytes)
	}
	return nil
}

func stringClaimValues(raw json.RawMessage, allowArray bool) ([]string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var scalar string
	if err := json.Unmarshal(raw, &scalar); err == nil {
		if validateClaimValue(scalar) != nil {
			return nil, false
		}
		return []string{scalar}, true
	}
	if !allowArray {
		return nil, false
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
		return nil, false
	}
	for _, value := range values {
		if validateClaimValue(value) != nil {
			return nil, false
		}
	}
	return values, true
}

func decodeUniqueClaims(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("claims must be an object")
	}
	claims := make(map[string]json.RawMessage)
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return nil, errors.New("claim name must be a string")
		}
		if _, duplicate := claims[name]; duplicate {
			return nil, errors.New("duplicate claim")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		claims[name] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("claims object is incomplete")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("claims contain trailing data")
	}
	return claims, nil
}

func opaquePrincipalID(issuer, claimValue string) string {
	digest := sha256.Sum256([]byte("thinkpixelmp:oidc-principal:v1\x00" + issuer + "\x00" + claimValue))
	return "oidc:" + base64.RawURLEncoding.EncodeToString(digest[:])
}

var _ identity.Mapper = (*Mapper)(nil)
