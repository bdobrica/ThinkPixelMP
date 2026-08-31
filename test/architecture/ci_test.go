package architecture_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCIWorkflowIsLeastPrivilegeAndImmutable(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	workflow := string(contents)

	required := []string{
		"permissions:\n  contents: read",
		"persist-credentials: false",
		"GOTOOLCHAIN: go1.26.7",
		"npm ci --ignore-scripts",
		"run: make verify",
		"run: make image",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("CI workflow is missing %q", fragment)
		}
	}

	for _, forbidden := range []string{"contents: write", "id-token: write", "pull-requests: write", "secrets."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("CI workflow contains forbidden privilege or secret reference %q", forbidden)
		}
	}

	actionUse := regexp.MustCompile(`(?m)^\s*- uses:\s*([^\s]+)(?:\s+#.*)?$`)
	immutableAction := regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)
	uses := actionUse.FindAllStringSubmatch(workflow, -1)
	if len(uses) == 0 {
		t.Fatal("CI workflow contains no actions")
	}
	for _, use := range uses {
		if !immutableAction.MatchString(use[1]) {
			t.Errorf("action is not pinned to a full commit SHA: %s", use[1])
		}
	}
}
