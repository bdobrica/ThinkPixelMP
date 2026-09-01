// Command repositoryhygiene rejects tracked files that look like committed secrets
// or private evidence. It deliberately scans the Git index rather than ignored or
// untracked developer files.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	privateKey = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
	jwt        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	dockerAuth = regexp.MustCompile(`(?i)"auth"\s*:\s*"[A-Za-z0-9+/]{16,}={0,2}"`)
	secretLine = regexp.MustCompile(`(?i)(?:registry[_-]?(?:password|token)|oidc[_-]?(?:token|client[_-]?secret)|signing[_-]?(?:key|secret)|client[_-]?secret|test[_-]?secret)\s*[:=]\s*["']?([^\s"']{16,})`)
)

type finding struct {
	path   string
	reason string
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fail(err)
	}
	paths, err := trackedFiles(root)
	if err != nil {
		fail(err)
	}
	findings := scan(root, paths)
	if len(findings) != 0 {
		for _, item := range findings {
			fmt.Fprintf(os.Stderr, "%s: %s\n", item.path, item.reason)
		}
		fmt.Fprintln(os.Stderr, "repository hygiene failed: remove the material and rotate any real credential")
		os.Exit(1)
	}
	fmt.Printf("Repository hygiene passed (%d tracked files scanned).\n", len(paths))
}

func repositoryRoot() (string, error) {
	command := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("locate repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func trackedFiles(root string) ([]string, error) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	items := bytes.Split(output, []byte{0})
	paths := make([]string, 0, len(items))
	for _, item := range items {
		if len(item) != 0 {
			paths = append(paths, string(item))
		}
	}
	return paths, nil
}

func scan(root string, paths []string) []finding {
	var findings []finding
	for _, path := range paths {
		if reason := prohibitedPath(path); reason != "" {
			findings = append(findings, finding{path: path, reason: reason})
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			findings = append(findings, finding{path: path, reason: "cannot inspect tracked file"})
			continue
		}
		if bytes.IndexByte(content, 0) >= 0 {
			continue
		}
		for _, pattern := range []struct {
			expression *regexp.Regexp
			reason     string
		}{
			{privateKey, "contains private signing key material"},
			{jwt, "contains a JWT/OIDC token"},
			{dockerAuth, "contains an encoded registry credential"},
			{secretLine, "contains an assigned credential or test secret"},
		} {
			if pattern.expression.Find(content) != nil {
				findings = append(findings, finding{path: path, reason: pattern.reason})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path == findings[j].path {
			return findings[i].reason < findings[j].reason
		}
		return findings[i].path < findings[j].path
	})
	return findings
}

func prohibitedPath(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if base == ".env.example" {
		return ""
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == "credentials.json" || base == "auth.json" || base == "dockerconfigjson" {
		return "credential-bearing filename is prohibited"
	}
	for _, suffix := range []string{".key", ".pem", ".p12", ".pfx", ".jks"} {
		if strings.HasSuffix(base, suffix) {
			return "private key or keystore file is prohibited"
		}
	}
	for _, segment := range strings.Split(lower, "/") {
		if segment == "private-evidence" || segment == "raw-evidence" || segment == "secret-testdata" || segment == "test-secrets" {
			return "private evidence or test-secret directory is prohibited"
		}
	}
	return ""
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "repository hygiene:", err)
	os.Exit(2)
}
