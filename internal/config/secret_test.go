package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretReferenceParsing(t *testing.T) {
	for _, value := range []string{"env:NAME", "env:_NAME_2", "file:/run/secrets/database"} {
		if _, err := ParseSecretRef(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"", "NAME", "literal:secret", "env:BAD-NAME", "env:", "file:relative", "file:/tmp/../secret", "https://example.test/secret"} {
		if _, err := ParseSecretRef(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestResolveEnvironmentSecret(t *testing.T) {
	const canary = "SECRET_VALUE_CANARY_1234"
	ref, _ := ParseSecretRef("env:DATABASE_DSN")
	secret, err := ref.Resolve(func(name string) (string, bool) { return canary, name == "DATABASE_DSN" })
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value() != canary {
		t.Fatal("wrong secret")
	}
	assertSecretRedacted(t, canary, secret)
	if _, err := ref.Resolve(func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("expected unavailable error")
	}
}

func TestResolveFileSecret(t *testing.T) {
	const canary = "FILE_SECRET_CANARY_5289"
	name := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(name, []byte(canary), 0o600); err != nil {
		t.Fatal(err)
	}
	ref, err := ParseSecretRef("file:" + name)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := ref.Resolve(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value() != canary {
		t.Fatal("file content changed")
	}
	assertSecretRedacted(t, canary, secret)
	directory, _ := ParseSecretRef("file:" + t.TempDir())
	if _, err := directory.Resolve(os.LookupEnv); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestResolveRejectsEmptyAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		body []byte
	}{{"empty", nil}, {"large", make([]byte, maximumSecretSize+1)}} {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(dir, tc.name)
			if err := os.WriteFile(name, tc.body, 0o600); err != nil {
				t.Fatal(err)
			}
			ref, _ := ParseSecretRef("file:" + name)
			if _, err := ref.Resolve(os.LookupEnv); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSecretReferenceErrorsDoNotEchoInput(t *testing.T) {
	const canary = "SECRET_REF_CANARY_8812"
	_, err := ParseSecretRef("unsupported:" + canary)
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("unsafe error: %v", err)
	}
	ref, _ := ParseSecretRef("env:" + canary)
	_, err = ref.Resolve(func(string) (string, bool) { return "", false })
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("unsafe resolution error: %v", err)
	}
}

func assertSecretRedacted(t *testing.T, canary string, secret Secret) {
	t.Helper()
	b, err := json.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}
	for _, rendered := range []string{secret.String(), fmt.Sprintf("%v", secret), fmt.Sprintf("%#v", secret), string(b)} {
		if strings.Contains(rendered, canary) {
			t.Fatalf("secret leaked: %s", rendered)
		}
		if !strings.Contains(rendered, redactedMarker) {
			t.Fatalf("marker absent: %s", rendered)
		}
	}
}
