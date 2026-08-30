// Command dependencycheck validates the selected Go module graph against repository policy.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const policyFilename = "dependency-policy.json"

var (
	exactVersionPattern  = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	pseudoVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]*[0-9]{14}-[0-9a-f]{12,}(?:\+incompatible)?$`)
)

type policy struct {
	SchemaVersion                 int         `json:"schema_version"`
	AllowedModulePrefixes         []string    `json:"allowed_module_prefixes"`
	AllowedLicenses               []string    `json:"allowed_licenses"`
	ForbiddenGoDirectives         []string    `json:"forbidden_go_directives"`
	AllowVendor                   bool        `json:"allow_vendor"`
	RequirePublicChecksumDatabase bool        `json:"require_public_checksum_database"`
	AllowPrivateModules           bool        `json:"allow_private_modules"`
	MaximumExceptionDays          int         `json:"maximum_exception_days"`
	Exceptions                    []exception `json:"exceptions"`
}

type exception struct {
	Kind                 string `json:"kind"`
	Module               string `json:"module"`
	Version              string `json:"version"`
	License              string `json:"license,omitempty"`
	Finding              string `json:"finding,omitempty"`
	Owner                string `json:"owner"`
	Justification        string `json:"justification"`
	Approval             string `json:"approval"`
	CreatedOn            string `json:"created_on"`
	ExpiresOn            string `json:"expires_on"`
	CompensatingControls string `json:"compensating_controls"`
	RemovalPlan          string `json:"removal_plan"`
}

type listedModule struct {
	Path      string        `json:"Path"`
	Version   string        `json:"Version"`
	Main      bool          `json:"Main"`
	Replace   *listedModule `json:"Replace"`
	Retracted []string      `json:"Retracted"`
	Error     *moduleError  `json:"Error"`
}

type moduleError struct {
	Err string `json:"Err"`
}

type commandRunner interface {
	Output(context.Context, ...string) ([]byte, error)
}

type goRunner struct {
	directory string
}

func (runner goRunner) Output(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), arguments...)
	command.Dir = runner.directory
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func main() {
	root := flag.String("root", ".", "repository root containing go.mod and dependency-policy.json")
	flag.Parse()

	if err := check(context.Background(), *root, time.Now().UTC(), goRunner{directory: *root}); err != nil {
		fmt.Fprintf(os.Stderr, "dependency policy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("dependency policy: valid")
}

func check(ctx context.Context, root string, now time.Time, runner commandRunner) error {
	configured, err := loadPolicy(filepath.Join(root, policyFilename))
	if err != nil {
		return err
	}
	if err := validatePolicy(configured, now); err != nil {
		return err
	}
	if err := validateGoMod(filepath.Join(root, "go.mod"), configured.ForbiddenGoDirectives); err != nil {
		return err
	}
	if !configured.AllowVendor {
		if info, err := os.Stat(filepath.Join(root, "vendor")); err == nil && info.IsDir() {
			return errors.New("vendor directory is forbidden")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect vendor directory: %w", err)
		}
	}

	modules, err := listModules(ctx, runner)
	if err != nil {
		return err
	}
	if err := validateModules(modules, configured, now); err != nil {
		return err
	}
	if err := validateChecksumPosture(ctx, root, runner, configured, modules); err != nil {
		return err
	}
	return nil
}

func loadPolicy(path string) (policy, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return policy{}, fmt.Errorf("read policy: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var configured policy
	if err := decoder.Decode(&configured); err != nil {
		return policy{}, fmt.Errorf("decode policy: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return policy{}, err
	}
	return configured, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing policy content: %w", err)
	}
	return errors.New("policy contains multiple JSON values")
}

func validatePolicy(configured policy, now time.Time) error {
	if configured.SchemaVersion != 1 {
		return fmt.Errorf("unsupported policy schema_version %d", configured.SchemaVersion)
	}
	if configured.MaximumExceptionDays != 90 {
		return fmt.Errorf("maximum_exception_days must be 90, got %d", configured.MaximumExceptionDays)
	}
	if configured.AllowVendor {
		return errors.New("allow_vendor must be false")
	}
	if !configured.RequirePublicChecksumDatabase {
		return errors.New("require_public_checksum_database must be true")
	}
	if configured.AllowPrivateModules {
		return errors.New("allow_private_modules must be false")
	}
	if err := validateSortedUnique("allowed_module_prefixes", configured.AllowedModulePrefixes); err != nil {
		return err
	}
	for _, prefix := range configured.AllowedModulePrefixes {
		if !strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, "*? ") {
			return fmt.Errorf("allowed module prefix %q must be an exact slash-terminated prefix without wildcard or whitespace", prefix)
		}
	}
	if err := validateSortedUnique("allowed_licenses", configured.AllowedLicenses); err != nil {
		return err
	}
	if err := validateSortedUnique("forbidden_go_directives", configured.ForbiddenGoDirectives); err != nil {
		return err
	}
	if strings.Join(configured.ForbiddenGoDirectives, ",") != "exclude,replace" {
		return errors.New("forbidden_go_directives must contain exactly exclude and replace")
	}
	seenExceptions := make(map[string]struct{}, len(configured.Exceptions))
	for index, exception := range configured.Exceptions {
		if err := validateException(exception, configured, now); err != nil {
			return fmt.Errorf("exception %d: %w", index, err)
		}
		key := strings.Join([]string{exception.Kind, exception.Module, exception.Version, exception.License, exception.Finding}, "\x00")
		if _, exists := seenExceptions[key]; exists {
			return fmt.Errorf("exception %d duplicates an earlier exact scope", index)
		}
		seenExceptions[key] = struct{}{}
	}
	return nil
}

func validateSortedUnique(name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	for index, value := range values {
		if strings.TrimSpace(value) != value || value == "" {
			return fmt.Errorf("%s contains an empty or non-canonical value", name)
		}
		if index > 0 && values[index-1] >= value {
			return fmt.Errorf("%s must be strictly sorted and unique", name)
		}
	}
	return nil
}

func validateException(candidate exception, configured policy, now time.Time) error {
	validKinds := map[string]bool{"source": true, "pseudo-version": true, "license": true, "vulnerability": true}
	if !validKinds[candidate.Kind] {
		return fmt.Errorf("invalid kind %q", candidate.Kind)
	}
	required := map[string]string{
		"module": candidate.Module, "version": candidate.Version, "owner": candidate.Owner,
		"justification": candidate.Justification, "approval": candidate.Approval,
		"compensating_controls": candidate.CompensatingControls, "removal_plan": candidate.RemovalPlan,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "*?") {
			return fmt.Errorf("%s must be exact, non-empty, and contain no wildcard", name)
		}
	}
	if candidate.Kind == "license" && strings.TrimSpace(candidate.License) == "" {
		return errors.New("license exception requires license")
	}
	if candidate.Kind == "vulnerability" && strings.TrimSpace(candidate.Finding) == "" {
		return errors.New("vulnerability exception requires finding")
	}
	created, err := time.Parse(time.DateOnly, candidate.CreatedOn)
	if err != nil {
		return fmt.Errorf("invalid created_on: %w", err)
	}
	expires, err := time.Parse(time.DateOnly, candidate.ExpiresOn)
	if err != nil {
		return fmt.Errorf("invalid expires_on: %w", err)
	}
	today := now.UTC().Truncate(24 * time.Hour)
	if created.After(today) {
		return errors.New("created_on is in the future")
	}
	if expires.Before(today) {
		return errors.New("exception is expired")
	}
	if expires.Before(created) || expires.Sub(created) > time.Duration(configured.MaximumExceptionDays)*24*time.Hour {
		return fmt.Errorf("exception lifetime exceeds %d days", configured.MaximumExceptionDays)
	}
	return nil
}

func validateGoMod(path string, forbidden []string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	for lineNumber, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(strings.SplitN(line, "//", 2)[0])
		if len(fields) == 0 {
			continue
		}
		for _, directive := range forbidden {
			if fields[0] == directive {
				return fmt.Errorf("go.mod line %d uses forbidden %s directive", lineNumber+1, directive)
			}
		}
	}
	return nil
}

func listModules(ctx context.Context, runner commandRunner) ([]listedModule, error) {
	output, err := runner.Output(ctx, "list", "-m", "-json", "-retracted", "all")
	if err != nil {
		return nil, fmt.Errorf("resolve module graph: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var modules []listedModule
	for {
		var module listedModule
		if err := decoder.Decode(&module); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode module graph: %w", err)
		}
		modules = append(modules, module)
	}
	if len(modules) == 0 {
		return nil, errors.New("module graph is empty")
	}
	return modules, nil
}

func validateModules(modules []listedModule, configured policy, now time.Time) error {
	for _, module := range modules {
		if module.Error != nil {
			return fmt.Errorf("module %s has resolution error: %s", module.Path, module.Error.Err)
		}
		if module.Replace != nil {
			return fmt.Errorf("module %s is replaced", module.Path)
		}
		if len(module.Retracted) > 0 {
			return fmt.Errorf("module %s@%s is retracted: %s", module.Path, module.Version, strings.Join(module.Retracted, "; "))
		}
		if module.Main {
			continue
		}
		if module.Path == "" || !exactVersionPattern.MatchString(module.Version) {
			return fmt.Errorf("module has non-exact identity %s@%s", module.Path, module.Version)
		}
		if !hasAllowedPrefix(module.Path, configured.AllowedModulePrefixes) && !hasException(configured, "source", module.Path, module.Version, now) {
			return fmt.Errorf("module source is not allowed: %s@%s", module.Path, module.Version)
		}
		if pseudoVersionPattern.MatchString(module.Version) && !hasException(configured, "pseudo-version", module.Path, module.Version, now) {
			return fmt.Errorf("pseudo-version requires an active exact exception: %s@%s", module.Path, module.Version)
		}
	}
	return nil
}

func hasAllowedPrefix(modulePath string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(modulePath, prefix) {
			return true
		}
	}
	return false
}

func hasException(configured policy, kind, modulePath, version string, now time.Time) bool {
	for _, candidate := range configured.Exceptions {
		if candidate.Kind != kind || candidate.Module != modulePath || candidate.Version != version {
			continue
		}
		expires, err := time.Parse(time.DateOnly, candidate.ExpiresOn)
		if err == nil && !expires.Before(now.UTC().Truncate(24*time.Hour)) {
			return true
		}
	}
	return false
}

func validateChecksumPosture(ctx context.Context, root string, runner commandRunner, configured policy, modules []listedModule) error {
	if !configured.RequirePublicChecksumDatabase {
		return nil
	}
	output, err := runner.Output(ctx, "env", "GOSUMDB", "GOPRIVATE", "GONOSUMDB")
	if err != nil {
		return fmt.Errorf("read checksum database configuration: %w", err)
	}
	settings := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(settings) == 0 || strings.TrimSpace(settings[0]) == "off" {
		return errors.New("GOSUMDB=off violates public checksum policy")
	}
	privatePatterns := ""
	noSumPatterns := ""
	if len(settings) > 1 {
		privatePatterns = strings.TrimSpace(settings[1])
	}
	if len(settings) > 2 {
		noSumPatterns = strings.TrimSpace(settings[2])
	}
	for _, module := range modules {
		if module.Main {
			continue
		}
		if matchesModulePatterns(privatePatterns, module.Path) || matchesModulePatterns(noSumPatterns, module.Path) {
			return fmt.Errorf("module %s bypasses public checksum verification through GOPRIVATE or GONOSUMDB", module.Path)
		}
	}
	if len(modules) > 1 {
		if info, err := os.Stat(filepath.Join(root, "go.sum")); err != nil {
			return fmt.Errorf("third-party module graph requires go.sum: %w", err)
		} else if info.IsDir() {
			return errors.New("go.sum is a directory")
		}
	}
	if _, err := runner.Output(ctx, "mod", "verify"); err != nil {
		return fmt.Errorf("verify module checksums: %w", err)
	}
	return nil
}

func matchesModulePatterns(patterns, modulePath string) bool {
	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		prefix := modulePath
		if slashCount := strings.Count(pattern, "/"); slashCount < strings.Count(modulePath, "/") {
			parts := strings.Split(modulePath, "/")
			prefix = strings.Join(parts[:slashCount+1], "/")
		}
		if matched, err := path.Match(pattern, prefix); err == nil && matched {
			return true
		}
	}
	return false
}
