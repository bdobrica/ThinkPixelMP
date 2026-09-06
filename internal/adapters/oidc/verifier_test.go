package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
)

func TestVerifierValidatesSignatureAndRegisteredClaims(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key := generateKey(t)
	verifier := newTestVerifier(t, key, now, 30*time.Second)
	raw := signToken(t, key, "RS256", map[string]any{
		"iss": "https://issuer.example.test", "sub": "principal-7",
		"aud": []string{"thinkpixelmp", "another-client"},
		"azp": "thinkpixelmp",
		"exp": now.Add(time.Minute).Unix(), "nbf": now.Add(-time.Minute).Unix(),
		"tenant_claim": "tenant-a",
	})

	verified, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Issuer != "https://issuer.example.test" || verified.Subject != "principal-7" || !verified.ExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("unexpected verified token: %#v", verified)
	}
	if len(verified.Audiences) != 2 || verified.NotBefore == nil || !strings.Contains(string(verified.Claims), "tenant_claim") {
		t.Fatalf("verified claims were not retained: %#v", verified)
	}
}

func TestVerifierRejectsUntrustedOrInvalidTokens(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key, otherKey := generateKey(t), generateKey(t)
	verifier := newTestVerifier(t, key, now, 30*time.Second)
	valid := func() map[string]any {
		return map[string]any{"iss": "https://issuer.example.test", "sub": "principal-7", "aud": "thinkpixelmp", "exp": now.Add(time.Minute).Unix()}
	}
	tests := []struct {
		name  string
		key   *rsa.PrivateKey
		alg   string
		alter func(map[string]any)
	}{
		{"wrong signature", otherKey, "RS256", func(map[string]any) {}},
		{"wrong issuer", key, "RS256", func(c map[string]any) { c["iss"] = "https://attacker.example.test" }},
		{"wrong audience", key, "RS256", func(c map[string]any) { c["aud"] = "another-client" }},
		{"missing authorized party", key, "RS256", func(c map[string]any) { c["aud"] = []string{"thinkpixelmp", "another-client"} }},
		{"wrong authorized party", key, "RS256", func(c map[string]any) {
			c["aud"] = []string{"thinkpixelmp", "another-client"}
			c["azp"] = "another-client"
		}},
		{"algorithm substitution", key, "RS512", func(map[string]any) {}},
		{"expired beyond skew", key, "RS256", func(c map[string]any) { c["exp"] = now.Add(-31 * time.Second).Unix() }},
		{"not active beyond skew", key, "RS256", func(c map[string]any) { c["nbf"] = now.Add(31 * time.Second).Unix() }},
		{"missing expiry", key, "RS256", func(c map[string]any) { delete(c, "exp") }},
		{"missing subject", key, "RS256", func(c map[string]any) { delete(c, "sub") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := valid()
			tc.alter(claims)
			_, err := verifier.Verify(context.Background(), signToken(t, tc.key, tc.alg, claims))
			assertTypedError(t, err, shared.ErrorUnauthorized, "identity.invalid_token")
		})
	}
}

func TestVerifierRequiresLiteralIssuerMatch(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key := generateKey(t)
	configured := testConfig(0)
	configured.Issuer = "https://accounts.google.com"
	keys := &coreoidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	verifier, err := New(configured, keys, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	raw := signToken(t, key, "RS256", map[string]any{"iss": "accounts.google.com", "sub": "p", "aud": configured.Audience, "exp": now.Add(time.Minute).Unix()})
	_, err = verifier.Verify(context.Background(), raw)
	assertTypedError(t, err, shared.ErrorUnauthorized, "identity.invalid_token")
}

func TestVerifierAppliesBoundedClockSkew(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key := generateKey(t)
	verifier := newTestVerifier(t, key, now, 30*time.Second)
	for name, claims := range map[string]map[string]any{
		"recently expired": {"iss": "https://issuer.example.test", "sub": "p", "aud": "thinkpixelmp", "exp": now.Add(-29 * time.Second).Unix()},
		"nearly active":    {"iss": "https://issuer.example.test", "sub": "p", "aud": "thinkpixelmp", "exp": now.Add(time.Minute).Unix(), "nbf": now.Add(29 * time.Second).Unix()},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.Verify(context.Background(), signToken(t, key, "RS256", claims)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifierBoundsInput(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	key := generateKey(t)
	verifier := newTestVerifier(t, key, now, 0)
	_, err := verifier.Verify(context.Background(), strings.Repeat("x", maximumTokenBytes+1))
	assertTypedError(t, err, shared.ErrorUnauthorized, "identity.invalid_token")
}

func TestNewRejectsUnsafeConfiguration(t *testing.T) {
	key := generateKey(t)
	keys := &coreoidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	now := clock.Fixed{Time: time.Now()}
	for name, mutate := range map[string]func(*Config){
		"http issuer":         func(c *Config) { c.Issuer = "http://issuer.example.test" },
		"missing audience":    func(c *Config) { c.Audience = "" },
		"missing algorithms":  func(c *Config) { c.AllowedAlgorithms = nil },
		"excessive skew":      func(c *Config) { c.ClockSkew = 5*time.Minute + time.Nanosecond },
		"unbounded discovery": func(c *Config) { c.DiscoveryTimeout = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			configured := testConfig(30 * time.Second)
			mutate(&configured)
			if _, err := New(configured, keys, now); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func newTestVerifier(t *testing.T, key *rsa.PrivateKey, now time.Time, skew time.Duration) *Verifier {
	t.Helper()
	keys := &coreoidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	verifier, err := New(testConfig(skew), keys, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func testConfig(skew time.Duration) Config {
	return Config{Issuer: "https://issuer.example.test", Audience: "thinkpixelmp", AllowedAlgorithms: []string{"RS256"}, ClockSkew: skew, DiscoveryTimeout: time.Second}
}

func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signToken(t *testing.T, key *rsa.PrivateKey, algorithm string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": algorithm, "kid": "test-key", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(encoded))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func assertTypedError(t *testing.T, err error, class shared.ErrorClass, code string) {
	t.Helper()
	var typed *shared.TypedError
	if !errors.As(err, &typed) || typed.Class() != class || typed.Code().String() != code {
		t.Fatalf("error = %v, want %s:%s", err, class, code)
	}
}
