package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRejectsRestrictedMaterial(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"registry.json":         `{"au` + `th":"dXNlcjphLXJlYWwtcGFzc3dvcmQ="}`,
		"signer.txt":            "-----BEGIN PRIVATE" + " KEY-----\nnot-a-real-key\n",
		"identity.txt":          "eyJhbGciOiJSUzI1NiJ9." + "eyJzdWIiOiJwcm9kdWNlciJ9." + "c2lnbmF0dXJlMTIzNA",
		"config.txt":            "oidc_client_" + "secret=real-looking-secret-value",
		"raw-evidence/data.txt": "confidential report",
	}
	paths := make([]string, 0, len(files))
	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	findings := scan(root, paths)
	if len(findings) != len(files) {
		t.Fatalf("got %d findings, want %d: %#v", len(findings), len(files), findings)
	}
}

func TestScanAllowsPublicEvidenceAndObviousPlaceholders(t *testing.T) {
	root := t.TempDir()
	path := "docs/evidence/example.md"
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "Verification passed. Use TPMP_POSTGRES_PASSWORD='<local development secret>'."
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if findings := scan(root, []string{path}); len(findings) != 0 {
		t.Fatalf("unexpected findings: %#v", findings)
	}
}

func TestProhibitedCredentialPaths(t *testing.T) {
	for _, path := range []string{".env", ".env.production", "deploy/credentials.json", "keys/signing.pem", "test/test-secrets/value"} {
		if prohibitedPath(path) == "" {
			t.Errorf("expected %q to be prohibited", path)
		}
	}
	for _, path := range []string{".env.example", "docs/security/registry-credentials.md", "docs/evidence/release.md", "public/signing-key.txt"} {
		if reason := prohibitedPath(path); reason != "" {
			t.Errorf("expected %q to be allowed, got %q", path, reason)
		}
	}
}
