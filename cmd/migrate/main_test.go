package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"down"}, {"up", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v: code %d", args, code)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "status|up") {
		t.Fatal("help failed")
	}
}

func TestRunRequiresSeparateSecretAndRedactsErrors(t *testing.T) {
	for _, command := range []string{"status", "up"} {
		for _, ref := range []string{"", "postgres://sensitive-value", "env:TPMP_TEST_MISSING", "env:TPMP_TEST_BAD_URL"} {
			t.Setenv("TPMP_MIGRATION_DATABASE_URL_REF", ref)
			t.Setenv("TPMP_TEST_MISSING", "")
			t.Setenv("TPMP_TEST_BAD_URL", "postgres://sensitive-value:%invalid")
			var stdout, stderr bytes.Buffer
			if code := run([]string{command}, &stdout, &stderr); code != 1 {
				t.Fatalf("code %d", code)
			}
			if strings.Contains(stderr.String(), "sensitive-value") || stdout.Len() != 0 {
				t.Fatal("secret leaked")
			}
		}
	}
}
