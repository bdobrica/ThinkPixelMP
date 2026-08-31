package architecture_test

import (
	"os"
	"strings"
	"testing"
)

func TestServiceContainerSecurityBaseline(t *testing.T) {
	contents, err := os.ReadFile("../../Containerfile")
	if err != nil {
		t.Fatalf("read Containerfile: %v", err)
	}
	file := string(contents)
	for _, required := range []string{
		"FROM golang:1.26.7-bookworm@sha256:",
		"CGO_ENABLED=0 GOOS=linux go build",
		"FROM scratch",
		"USER 65532:65532",
		`ENTRYPOINT ["/thinkpixelmp"]`,
	} {
		if !strings.Contains(file, required) {
			t.Errorf("Containerfile must contain %q", required)
		}
	}
	for _, forbidden := range []string{"USER root", "chmod 777", "TPMP_DATABASE_URL="} {
		if strings.Contains(file, forbidden) {
			t.Errorf("Containerfile must not contain %q", forbidden)
		}
	}
}
