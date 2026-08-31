package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunStatusReportsEmptyMigrationSet(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run status code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no executable migrations") {
		t.Fatalf("run status output = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("run status stderr = %q", stderr.String())
	}
}

func TestRunRejectsMutationBeforeMigrationFrameworkExists(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"up"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run up code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown migration command") {
		t.Fatalf("run up stderr = %q", stderr.String())
	}
}
