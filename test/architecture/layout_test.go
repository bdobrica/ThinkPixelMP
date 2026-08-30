package architecture_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/bdobrica/ThinkPixelMP"

func TestPlannedDirectoriesExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	directories := []string{
		"cmd/thinkpixelmp", "cmd/migrate", "cmd/thinkpixelmpctl",
		"api/openapi", "api/schemas",
		"internal/domain/publisher", "internal/domain/artifact", "internal/domain/evidence",
		"internal/domain/catalog", "internal/domain/promotion", "internal/domain/resolution", "internal/domain/revocation",
		"internal/app/publication", "internal/app/discovery", "internal/app/evidence",
		"internal/app/promotion", "internal/app/resolution", "internal/app/federation",
		"internal/ports/registry", "internal/ports/signature", "internal/ports/provenance",
		"internal/ports/evidence", "internal/ports/policy", "internal/ports/identity",
		"internal/ports/key", "internal/ports/importer", "internal/ports/clock",
		"internal/adapters/registry/oras", "internal/adapters/signature/sigstore", "internal/adapters/policy/opa",
		"internal/adapters/import/mcpregistry", "internal/adapters/import/oci",
		"internal/adapters/import/a2a", "internal/adapters/import/git",
		"internal/adapters/http", "internal/adapters/oidc", "internal/adapters/postgres",
		"internal/adapters/evidence", "internal/adapters/key", "internal/telemetry", "internal/security",
		"migrations", "deploy/helm", "docs/adr", "docs/contracts",
		"test/integration", "test/contract", "test/security", "test/federation", "test/e2e", "test/architecture",
	}

	for _, directory := range directories {
		directory := directory
		t.Run(directory, func(t *testing.T) {
			t.Parallel()

			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(directory)))
			if err != nil {
				t.Fatalf("planned directory %q: %v", directory, err)
			}
			if !info.IsDir() {
				t.Errorf("planned path %q is not a directory", directory)
			}
		})
	}
}

func TestPlannedGoPackagesAreDiscoverable(t *testing.T) {
	t.Parallel()

	want := []string{
		"cmd/migrate", "cmd/thinkpixelmp", "cmd/thinkpixelmpctl",
		"internal/adapters", "internal/adapters/evidence", "internal/adapters/http", "internal/adapters/import/a2a",
		"internal/adapters/import/git", "internal/adapters/import/mcpregistry", "internal/adapters/import/oci",
		"internal/adapters/key", "internal/adapters/oidc", "internal/adapters/policy", "internal/adapters/policy/opa",
		"internal/adapters/postgres", "internal/adapters/registry", "internal/adapters/registry/oras",
		"internal/adapters/signature", "internal/adapters/signature/sigstore",
		"internal/app", "internal/app/discovery", "internal/app/evidence", "internal/app/federation",
		"internal/app/promotion", "internal/app/publication", "internal/app/resolution",
		"internal/domain", "internal/domain/artifact", "internal/domain/catalog", "internal/domain/evidence",
		"internal/domain/promotion", "internal/domain/publisher", "internal/domain/resolution", "internal/domain/revocation",
		"internal/ports", "internal/ports/clock", "internal/ports/evidence", "internal/ports/identity",
		"internal/ports/importer", "internal/ports/key", "internal/ports/policy", "internal/ports/provenance",
		"internal/ports/registry", "internal/ports/signature", "internal/security", "internal/telemetry",
		"test/architecture", "test/contract", "test/e2e", "test/federation", "test/integration", "test/security",
	}
	for index := range want {
		want[index] = modulePath + "/" + want[index]
	}
	sort.Strings(want)

	command := goCommand(t, "list", "-f", "{{.ImportPath}}", "./...")
	command.Dir = repositoryRoot(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list packages: %v\n%s", err, output)
	}
	got := strings.Fields(string(output))
	discovered := make(map[string]struct{}, len(got))
	for _, importPath := range got {
		discovered[importPath] = struct{}{}
	}

	var missing []string
	for _, importPath := range want {
		if _, ok := discovered[importPath]; !ok {
			missing = append(missing, importPath)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("planned packages are not discoverable:\n%s", strings.Join(missing, "\n"))
	}
}

func TestDomainDoesNotImportAdapterTechnologies(t *testing.T) {
	t.Parallel()

	type listedPackage struct {
		ImportPath string
		Imports    []string
	}

	command := goCommand(t, "list", "-json", "./internal/domain/...")
	command.Dir = repositoryRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list domain packages: %v", err)
	}

	forbidden := []string{
		"oras.land/",
		"github.com/sigstore/",
		"github.com/open-policy-agent/opa",
		"github.com/jackc/pgx/",
		"github.com/go-chi/chi/",
		"github.com/gin-gonic/gin",
		"github.com/labstack/echo/",
		"github.com/gofiber/fiber/",
		"github.com/modelcontextprotocol/",
		"github.com/a2aproject/",
		"k8s.io/",
		"sigs.k8s.io/",
		"github.com/bdobrica/ThinkPixelAG/",
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		for _, imported := range pkg.Imports {
			for _, prefix := range forbidden {
				if strings.HasPrefix(imported, prefix) {
					t.Errorf("domain package %s imports forbidden adapter technology %s", pkg.ImportPath, imported)
				}
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func goCommand(t *testing.T, arguments ...string) *exec.Cmd {
	t.Helper()

	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), arguments...)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOMODCACHE="+t.TempDir())
	return command
}
