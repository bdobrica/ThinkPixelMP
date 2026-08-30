package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

type fakeRunner struct {
	outputs map[string]string
	errors  map[string]error
}

func (runner fakeRunner) Output(_ context.Context, arguments ...string) ([]byte, error) {
	key := strings.Join(arguments, " ")
	return []byte(runner.outputs[key]), runner.errors[key]
}

func TestRepositoryPolicyAndModuleGraph(t *testing.T) {
	root := repositoryRoot(t)
	if err := check(context.Background(), root, testNow, goRunner{directory: root}); err != nil {
		t.Fatalf("repository dependency policy: %v", err)
	}
}

func TestValidatePolicyRejectsInvalidSecurityPosture(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*policy)
		want   string
	}{
		{"schema", func(candidate *policy) { candidate.SchemaVersion = 2 }, "unsupported policy"},
		{"vendor", func(candidate *policy) { candidate.AllowVendor = true }, "allow_vendor"},
		{"checksum", func(candidate *policy) { candidate.RequirePublicChecksumDatabase = false }, "checksum"},
		{"private", func(candidate *policy) { candidate.AllowPrivateModules = true }, "private"},
		{"unsorted prefixes", func(candidate *policy) { candidate.AllowedModulePrefixes = []string{"oras.land/", "github.com/"} }, "sorted"},
		{"wildcard prefix", func(candidate *policy) { candidate.AllowedModulePrefixes = []string{"github.com/*/"} }, "wildcard"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := validPolicy()
			test.mutate(&candidate)
			if err := validatePolicy(candidate, testNow); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validatePolicy() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateExceptionScopeAndLifetime(t *testing.T) {
	valid := validException("pseudo-version")
	if err := validateException(valid, validPolicy(), testNow); err != nil {
		t.Fatalf("valid exception rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*exception)
		want   string
	}{
		{"wildcard", func(candidate *exception) { candidate.Module = "github.com/example/*" }, "wildcard"},
		{"expired", func(candidate *exception) { candidate.ExpiresOn = "2026-08-29" }, "expired"},
		{"too long", func(candidate *exception) { candidate.ExpiresOn = "2026-12-01" }, "exceeds"},
		{"future", func(candidate *exception) { candidate.CreatedOn = "2026-08-31" }, "future"},
		{"license detail", func(candidate *exception) { candidate.Kind = "license" }, "requires license"},
		{"finding detail", func(candidate *exception) { candidate.Kind = "vulnerability" }, "requires finding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateException(candidate, validPolicy(), testNow); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateException() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateModules(t *testing.T) {
	base := listedModule{Path: "github.com/example/module", Version: "v1.2.3"}
	tests := []struct {
		name   string
		module listedModule
		want   string
	}{
		{"allowed", base, ""},
		{"source", listedModule{Path: "example.invalid/module", Version: "v1.2.3"}, "source"},
		{"missing version", listedModule{Path: base.Path}, "non-exact"},
		{"malformed version", listedModule{Path: base.Path, Version: "vbranch"}, "non-exact"},
		{"replacement", listedModule{Path: base.Path, Version: base.Version, Replace: &listedModule{Path: "../local"}}, "replaced"},
		{"retracted", listedModule{Path: base.Path, Version: base.Version, Retracted: []string{"broken"}}, "retracted"},
		{"resolution", listedModule{Path: base.Path, Version: base.Version, Error: &moduleError{Err: "missing"}}, "resolution"},
		{"pseudo", listedModule{Path: base.Path, Version: "v0.0.0-20260830120000-0123456789ab"}, "pseudo-version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules := []listedModule{{Path: "github.com/bdobrica/ThinkPixelMP", Main: true}, test.module}
			err := validateModules(modules, validPolicy(), testNow)
			if test.want == "" && err != nil {
				t.Fatalf("allowed module rejected: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("validateModules() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestExactPseudoVersionException(t *testing.T) {
	configured := validPolicy()
	candidate := validException("pseudo-version")
	configured.Exceptions = []exception{candidate}
	modules := []listedModule{
		{Path: "github.com/bdobrica/ThinkPixelMP", Main: true},
		{Path: candidate.Module, Version: candidate.Version},
	}
	if err := validateModules(modules, configured, testNow); err != nil {
		t.Fatalf("exact active exception rejected: %v", err)
	}
	modules[1].Version = "v0.0.0-20260830120001-0123456789ac"
	if err := validateModules(modules, configured, testNow); err == nil {
		t.Fatal("different pseudo-version accepted by exact exception")
	}
}

func TestValidateGoModRejectsForbiddenDirectives(t *testing.T) {
	for _, directive := range []string{"replace", "exclude"} {
		t.Run(directive, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go.mod")
			contents := "module example.com/test\n\ngo 1.26.0\n\n" + directive + " example.com/old v1.0.0"
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateGoMod(path, []string{"exclude", "replace"}); err == nil || !strings.Contains(err.Error(), directive) {
				t.Fatalf("validateGoMod() error = %v", err)
			}
		})
	}
}

func TestLoadPolicyRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown":  `{"schema_version":1,"unknown":true}`,
		"trailing": `{}` + "\n{}",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadPolicy(path); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

func TestChecksumPosture(t *testing.T) {
	configured := validPolicy()
	runner := fakeRunner{outputs: map[string]string{"env GOSUMDB GOPRIVATE GONOSUMDB": "off\n\n\n", "mod verify": "all modules verified\n"}, errors: map[string]error{}}
	if err := validateChecksumPosture(context.Background(), t.TempDir(), runner, configured, []listedModule{{Main: true}}); err == nil || !strings.Contains(err.Error(), "GOSUMDB=off") {
		t.Fatalf("checksum policy error = %v", err)
	}

	runner.outputs["env GOSUMDB GOPRIVATE GONOSUMDB"] = "sum.golang.org\ngithub.com/example/*\n\n"
	modules := []listedModule{{Main: true}, {Path: "github.com/example/module", Version: "v1.0.0"}}
	if err := validateChecksumPosture(context.Background(), t.TempDir(), runner, configured, modules); err == nil || !strings.Contains(err.Error(), "bypasses") {
		t.Fatalf("checksum bypass error = %v", err)
	}
}

func validPolicy() policy {
	return policy{
		SchemaVersion: 1, AllowedModulePrefixes: []string{"github.com/", "oras.land/"},
		AllowedLicenses: []string{"Apache-2.0", "MIT"}, ForbiddenGoDirectives: []string{"exclude", "replace"},
		RequirePublicChecksumDatabase: true, MaximumExceptionDays: 90,
	}
}

func validException(kind string) exception {
	return exception{
		Kind: kind, Module: "github.com/example/module", Version: "v0.0.0-20260830120000-0123456789ab",
		Owner: "maintainers", Justification: "upstream has no tagged security fix", Approval: "SEC-001",
		CreatedOn: "2026-08-30", ExpiresOn: "2026-09-30", CompensatingControls: "feature disabled",
		RemovalPlan: "upgrade to tagged release",
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func marshalModules(t *testing.T, modules ...listedModule) string {
	t.Helper()

	var output strings.Builder
	encoder := json.NewEncoder(&output)
	for _, module := range modules {
		if err := encoder.Encode(module); err != nil {
			t.Fatal(err)
		}
	}
	return output.String()
}
