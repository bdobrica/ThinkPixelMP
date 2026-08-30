package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsAreSafeAndValid(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModeDevelopment || cfg.HTTP.Address != "127.0.0.1:8080" {
		t.Fatalf("unexpected defaults: %s", cfg)
	}
	if cfg.Database.URL.IsSet() || cfg.Telemetry.Mode != "noop" {
		t.Fatalf("trust-bearing default enabled: %s", cfg)
	}
}

func TestLoadPrecedence(t *testing.T) {
	name := writeConfig(t, `{"http":{"address":"file:1"},"log":{"level":"warn"}}`)
	cfg, err := Load([]string{"--config", name, "--http-address=flag:3"}, []string{"TPMP_HTTP_ADDRESS=env:2"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Address != "flag:3" || cfg.Log.Level != "warn" {
		t.Fatalf("precedence failed: %s", cfg)
	}
}

func TestLoadRejectsUnknownAndMalformedInput(t *testing.T) {
	tests := []struct {
		name      string
		args, env []string
		file      string
	}{
		{"unknown file field", nil, nil, `{"surprise":true}`},
		{"trailing JSON", nil, nil, `{} {}`},
		{"unknown environment", nil, []string{"TPMP_SURPRISE=true"}, ""},
		{"duplicate environment", nil, []string{"TPMP_LOG_LEVEL=info", "TPMP_LOG_LEVEL=warn"}, ""},
		{"unknown flag", []string{"--surprise=true"}, nil, ""},
		{"positional argument", []string{"surprise"}, nil, ""},
		{"bad duration", nil, []string{"TPMP_HTTP_READ_TIMEOUT=soon"}, ""},
		{"inline database secret", nil, []string{"TPMP_DATABASE_URL=postgres://user:pass@db/app"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.file != "" {
				args = []string{"--config", writeConfig(t, tc.file)}
			}
			if _, err := Load(args, tc.env); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadRejectsOversizedConfigurationFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(name, []byte(`{"padding":"`+strings.Repeat("x", maximumConfigFileSize)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{"--config", name}, nil); err == nil {
		t.Fatal("expected size-limit error")
	}
}

func TestProductionRequiresDatabaseSecretReference(t *testing.T) {
	if _, err := Load([]string{"--mode=production"}, nil); err == nil {
		t.Fatal("expected missing database reference error")
	}
	cfg, err := Load([]string{"--mode=production", "--database-url=env:TPMP_DATABASE_DSN"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.URL.Source() != SecretSourceEnvironment {
		t.Fatal("environment source not retained")
	}
}

func TestValidationBounds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"mode", func(c *Config) { c.Mode = "unsafe" }},
		{"address", func(c *Config) { c.HTTP.Address = "anywhere" }},
		{"body", func(c *Config) { c.HTTP.MaxBodyBytes = 16<<20 + 1 }},
		{"connections", func(c *Config) { c.Database.MinConnections = 21 }},
		{"log level", func(c *Config) { c.Log.Level = "verbose" }},
		{"sample ratio", func(c *Config) { c.Telemetry.SampleRatio = 2 }},
		{"endpoint credentials", func(c *Config) { c.Telemetry.Mode = "otlp"; c.Telemetry.Endpoint = "https://user:secret@example.test" }},
		{"endpoint query", func(c *Config) { c.Telemetry.Mode = "otlp"; c.Telemetry.Endpoint = "https://example.test?token=secret" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Defaults()
			tc.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestConfigurationRenderingRedactsReferences(t *testing.T) {
	const canary = "SECRET_REFERENCE_CANARY_9471"
	ref, err := ParseSecretRef("env:" + canary)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Database.URL = ref
	cfg.ConfigFile = "/private/" + canary + ".json"
	renderings := []string{cfg.String(), fmt.Sprintf("%v", cfg), fmt.Sprintf("%#v", cfg), fmt.Sprintf("%v", ref), fmt.Sprintf("%#v", ref)}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	renderings = append(renderings, string(b))
	for _, rendered := range renderings {
		if strings.Contains(rendered, canary) || strings.Contains(rendered, "/private/") {
			t.Fatalf("configuration leaked: %s", rendered)
		}
	}
	if !strings.Contains(cfg.String(), `"source":"environment"`) {
		t.Fatalf("safe source absent: %s", cfg)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}
