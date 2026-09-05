//go:build dbintegration

package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	postgresartifact "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifact"
	postgresartifactdependency "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactdependency"
	postgresartifactdescriptor "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactdescriptor"
	postgresartifactrequirement "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactrequirement"
	postgresartifactsource "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactsource"
	postgresartifactversion "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactversion"
	postgresnamespace "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/namespace"
	postgrespublisher "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/publisher"
	domainartifact "github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	domainartifactdependency "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdependency"
	domainartifactdescriptor "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdescriptor"
	domainartifactrequirement "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactrequirement"
	domainartifactsource "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
	domainartifactversion "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	domainnamespace "github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	domainpublisher "github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/migrations"
	"github.com/jackc/pgx/v5"
)

// This suite always uses its own disposable pinned PostgreSQL container. It never
// connects to an operator-supplied database or removes an existing container.
func TestPostgres(t *testing.T) {
	out, err := exec.Command("docker", "run", "--detach", "--rm", "--publish", "127.0.0.1::5432", "--env", "POSTGRES_HOST_AUTH_METHOD=trust", "postgres:18.6-bookworm").CombinedOutput()
	if err != nil {
		t.Fatalf("start PostgreSQL: %v: %s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		if out, err := exec.Command("docker", "rm", "--force", id).CombinedOutput(); err != nil {
			t.Errorf("cleanup: %v: %s", err, out)
		}
	})
	out, err = exec.Command("docker", "port", id, "5432/tcp").Output()
	if err != nil {
		t.Fatal(err)
	}
	address := strings.TrimSpace(string(out))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	connect := func(db string) (*pgx.Conn, error) {
		return pgx.Connect(ctx, "postgres://postgres@"+address+"/"+db+"?sslmode=disable")
	}
	var admin *pgx.Conn
	for ctx.Err() == nil {
		admin, err = connect("postgres")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("PostgreSQL did not become ready")
	}
	defer func() { _ = admin.Close(context.Background()) }()
	var version string
	if err := admin.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil || !strings.HasPrefix(version, "18.6") {
		t.Fatalf("unexpected PostgreSQL version %q: %v", version, err)
	}
	newDB := func(t *testing.T, name string) *pgx.Conn {
		t.Helper()
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
			t.Fatal(err)
		}
		conn, err := connect(name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close(context.Background()) })
		return conn
	}
	t.Run("empty_repeat_and_RLS", func(t *testing.T) {
		conn := newDB(t, "tenant_test")
		states, err := Run(ctx, conn, migrations.Files, false)
		if err != nil || len(states) != 10 || states[0].Applied || states[1].Applied || states[2].Applied || states[3].Applied || states[4].Applied || states[5].Applied || states[6].Applied || states[7].Applied || states[8].Applied || states[9].Applied {
			t.Fatalf("empty status: %v %v", states, err)
		}
		var exists bool
		_ = conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists)
		if exists {
			t.Fatal("status wrote a ledger")
		}
		for range 2 {
			if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
				t.Fatal(err)
			}
		}
		states, err = Run(ctx, conn, migrations.Files, false)
		if err != nil || !states[0].Applied || !states[1].Applied || !states[2].Applied || !states[3].Applied || !states[4].Applied || !states[5].Applied || !states[6].Applied || !states[7].Applied || !states[8].Applied || !states[9].Applied {
			t.Fatalf("applied status: %v %v", states, err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		a, b := "0198fc21-ced5-7000-8000-000000000000", "0198fc21-ced5-7000-8000-000000000001"
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", a, b))
		for _, invalid := range []string{"00000000-0000-4000-8000-000000000000", "00000000-0000-7000-0000-000000000000"} {
			if _, err := conn.Exec(ctx, "INSERT INTO public.tenants (tenant_id) VALUES ($1)", invalid); err == nil {
				t.Fatal("accepted non-v7 tenant")
			}
		}
		execSQL("CREATE ROLE db001_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT, INSERT, UPDATE, DELETE ON public.tenants TO db001_service")
		execSQL("SET ROLE db001_service")
		count := func(want int) {
			t.Helper()
			var n int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.tenants").Scan(&n); err != nil || n != want {
				t.Fatalf("tenant visibility = %d, want %d: %v", n, want, err)
			}
		}
		count(0)
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "SELECT set_config('thinkpixelmp.tenant_id', $1, true)", a); err != nil {
			t.Fatal(err)
		}
		var got string
		if err := tx.QueryRow(ctx, "SELECT tenant_id::text FROM public.tenants").Scan(&got); err != nil || got != a {
			t.Fatalf("tenant scope: %s %v", got, err)
		}
		result, err := tx.Exec(ctx, "DELETE FROM public.tenants WHERE tenant_id = $1", b)
		if err != nil || result.RowsAffected() != 0 {
			t.Fatal("cross-tenant delete allowed")
		}
		if _, err := tx.Exec(ctx, "UPDATE public.tenants SET tenant_id = $1 WHERE tenant_id = $2", b, a); err == nil {
			t.Fatal("cross-tenant write allowed")
		}
		_ = tx.Rollback(ctx)
		count(0)
		execSQL("RESET ROLE")
		execSQL("UPDATE public.schema_migrations SET checksum = repeat('0', 64)")
		for _, apply := range []bool{false, true} {
			if _, err := Run(ctx, conn, migrations.Files, apply); err == nil {
				t.Fatal("accepted checksum drift")
			}
		}
	})
	t.Run("publisher_repository", func(t *testing.T) {
		conn := newDB(t, "publisher_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		tenantA := "0198fc21-ced5-7000-8000-000000000010"
		tenantB := "0198fc21-ced5-7000-8000-000000000011"
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantA, tenantB))
		execSQL("CREATE ROLE db002_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT ON public.tenants TO db002_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.publishers TO db002_service")
		execSQL("GRANT SELECT, INSERT ON public.publisher_state_records TO db002_service")
		execSQL("SET ROLE db002_service")

		repository, err := postgrespublisher.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		parse := func(value string) shared.UUID {
			parsed, parseErr := shared.ParseUUID(value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return parsed
		}
		a, b := parse(tenantA), parse(tenantB)
		publisherAID := parse("0198fc21-ced5-7000-8000-000000000020")
		publisherBID := parse("0198fc21-ced5-7000-8000-000000000021")
		now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
		publisherA, _ := domainpublisher.New(a, publisherAID, "acme", "Acme", "", now)
		publisherB, _ := domainpublisher.New(b, publisherBID, "acme", "Other tenant", "", now)
		if err := repository.Create(ctx, publisherA); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, publisherB); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, publisherA); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate create class = %q: %v", typedClass(err), err)
		}
		got, err := repository.GetBySlug(ctx, a, "acme")
		if err != nil || got.ID() != publisherAID || got.State() != domainpublisher.StateClaimed {
			t.Fatalf("get: %#v %v", got, err)
		}
		values, err := repository.List(ctx, a, nil, 50)
		if err != nil || len(values) != 1 || values[0].TenantID() != a {
			t.Fatalf("tenant list: %#v %v", values, err)
		}
		if _, err := repository.Get(ctx, a, publisherBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}
		reason, _ := shared.NewReasonCode("ownership.confirmed")
		got, err = repository.ChangeState(ctx, a, publisherAID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute))
		if err != nil || got.State() != domainpublisher.StateVerified || got.StateVersion() != 2 {
			t.Fatalf("change state: %#v %v", got, err)
		}
		if _, err := repository.ChangeState(ctx, a, publisherAID, domainpublisher.StateClaimed, reason, "invalid", now.Add(2*time.Minute)); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("invalid transition class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.publisher_state_records SET state = 'suspended' WHERE tenant_id = $1::uuid`, tenantA); err == nil {
			t.Fatal("append-only state record was mutable")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.publisher_state_records
  (tenant_id, publisher_id, version, state, reason_code, recorded_at)
 VALUES ($1::uuid, $2::uuid, 3, 'claimed', 'invalid.transition', $3)`, tenantA, publisherAID.String(), now.Add(2*time.Minute)); err == nil {
			t.Fatal("database accepted an invalid state transition")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.publishers SET slug = 'rewritten'
 WHERE tenant_id = $1::uuid AND publisher_id = $2::uuid`, tenantA, publisherAID.String()); err == nil {
			t.Fatal("database allowed publisher identity mutation")
		}
	})
	t.Run("namespace_repository", func(t *testing.T) {
		conn := newDB(t, "namespace_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		tenantA := "0198fc21-ced5-7000-8000-000000000030"
		tenantB := "0198fc21-ced5-7000-8000-000000000031"
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantA, tenantB))
		execSQL("CREATE ROLE db003_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT ON public.tenants TO db003_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.publishers TO db003_service")
		execSQL("GRANT SELECT, INSERT ON public.publisher_state_records TO db003_service")
		execSQL("GRANT SELECT, INSERT ON public.namespaces TO db003_service")
		execSQL("SET ROLE db003_service")

		parse := func(value string) shared.UUID {
			parsed, parseErr := shared.ParseUUID(value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return parsed
		}
		a, b := parse(tenantA), parse(tenantB)
		publisherAID := parse("0198fc21-ced5-7000-8000-000000000040")
		publisherBID := parse("0198fc21-ced5-7000-8000-000000000041")
		claimedPublisherID := parse("0198fc21-ced5-7000-8000-000000000042")
		namespaceAID := parse("0198fc21-ced5-7000-8000-000000000050")
		namespaceBID := parse("0198fc21-ced5-7000-8000-000000000051")
		now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
		publisherRepository, err := postgrespublisher.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		publisherA, _ := domainpublisher.New(a, publisherAID, "acme", "Acme", "", now)
		publisherB, _ := domainpublisher.New(b, publisherBID, "acme", "Other tenant", "", now)
		claimedPublisher, _ := domainpublisher.New(a, claimedPublisherID, "pending", "Pending", "", now)
		for _, value := range []domainpublisher.Publisher{publisherA, publisherB, claimedPublisher} {
			if err := publisherRepository.Create(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
		reason, _ := shared.NewReasonCode("ownership.confirmed")
		if _, err := publisherRepository.ChangeState(ctx, a, publisherAID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, b, publisherBID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}

		repository, err := postgresnamespace.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		namespaceA, _ := domainnamespace.New(a, namespaceAID, "acme/security", publisherAID, now.Add(2*time.Minute))
		namespaceB, _ := domainnamespace.New(b, namespaceBID, "acme/security", publisherBID, now.Add(2*time.Minute))
		if err := repository.Create(ctx, namespaceA); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, namespaceB); err != nil {
			t.Fatal(err)
		}
		duplicatePath, _ := domainnamespace.New(a, parse("0198fc21-ced5-7000-8000-000000000052"), "acme/security", publisherAID, now.Add(3*time.Minute))
		if err := repository.Create(ctx, duplicatePath); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate path class = %q: %v", typedClass(err), err)
		}
		unverifiedOwner, _ := domainnamespace.New(a, parse("0198fc21-ced5-7000-8000-000000000053"), "acme/pending", claimedPublisherID, now)
		if err := repository.Create(ctx, unverifiedOwner); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("unverified owner class = %q: %v", typedClass(err), err)
		}
		crossTenantOwner, _ := domainnamespace.New(a, parse("0198fc21-ced5-7000-8000-000000000054"), "acme/cross-tenant", publisherBID, now)
		if err := repository.Create(ctx, crossTenantOwner); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("cross-tenant owner class = %q: %v", typedClass(err), err)
		}
		got, err := repository.GetByPath(ctx, a, "acme/security")
		if err != nil || got.ID() != namespaceAID || got.OwnerPublisherID() != publisherAID {
			t.Fatalf("get: %#v %v", got, err)
		}
		values, err := repository.List(ctx, a, nil, 50)
		if err != nil || len(values) != 1 || values[0].TenantID() != a {
			t.Fatalf("tenant list: %#v %v", values, err)
		}
		if _, err := repository.Get(ctx, a, namespaceBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.namespaces SET path = 'rewritten'
 WHERE tenant_id = $1::uuid AND namespace_id = $2::uuid`, tenantA, namespaceAID.String()); err == nil {
			t.Fatal("database allowed namespace identity mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.namespaces
 WHERE tenant_id = $1::uuid AND namespace_id = $2::uuid`, tenantA, namespaceAID.String()); err == nil {
			t.Fatal("database allowed namespace deletion")
		}
	})
	t.Run("artifact_repository", func(t *testing.T) {
		conn := newDB(t, "artifact_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		parse := func(value string) shared.UUID {
			parsed, err := shared.ParseUUID(value)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
		tenantA := parse("0198fc21-ced5-7000-8000-000000000060")
		tenantB := parse("0198fc21-ced5-7000-8000-000000000061")
		publisherAID := parse("0198fc21-ced5-7000-8000-000000000070")
		publisherBID := parse("0198fc21-ced5-7000-8000-000000000071")
		namespaceAID := parse("0198fc21-ced5-7000-8000-000000000080")
		namespaceBID := parse("0198fc21-ced5-7000-8000-000000000081")
		artifactAID := parse("0198fc21-ced5-7000-8000-000000000090")
		artifactBID := parse("0198fc21-ced5-7000-8000-000000000091")
		now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantA, tenantB))
		execSQL("CREATE ROLE db004_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT ON public.tenants TO db004_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.publishers TO db004_service")
		execSQL("GRANT SELECT, INSERT ON public.publisher_state_records TO db004_service")
		execSQL("GRANT SELECT, INSERT ON public.namespaces TO db004_service")
		execSQL("GRANT SELECT, INSERT ON public.artifacts TO db004_service")
		execSQL("SET ROLE db004_service")

		publisherRepository, _ := postgrespublisher.NewRepository(conn)
		publisherA, _ := domainpublisher.New(tenantA, publisherAID, "acme", "Acme", "", now)
		publisherB, _ := domainpublisher.New(tenantB, publisherBID, "acme", "Other tenant", "", now)
		for _, value := range []domainpublisher.Publisher{publisherA, publisherB} {
			if err := publisherRepository.Create(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
		reason, _ := shared.NewReasonCode("ownership.confirmed")
		if _, err := publisherRepository.ChangeState(ctx, tenantA, publisherAID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, tenantB, publisherBID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		namespaceRepository, _ := postgresnamespace.NewRepository(conn)
		namespaceA, _ := domainnamespace.New(tenantA, namespaceAID, "acme/security", publisherAID, now.Add(2*time.Minute))
		namespaceB, _ := domainnamespace.New(tenantB, namespaceBID, "acme/security", publisherBID, now.Add(2*time.Minute))
		if err := namespaceRepository.Create(ctx, namespaceA); err != nil {
			t.Fatal(err)
		}
		if err := namespaceRepository.Create(ctx, namespaceB); err != nil {
			t.Fatal(err)
		}

		repository, _ := postgresartifact.NewRepository(conn)
		artifactA, _ := domainartifact.New(tenantA, artifactAID, namespaceAID, "acme/security", "reviewer", domainartifact.KindSkill, "Reviewer", "Checks changes", "https://example.test/reviewer", "https://example.test/source", map[string]string{"security/tier": "reviewed"}, now.Add(3*time.Minute))
		artifactB, _ := domainartifact.New(tenantB, artifactBID, namespaceBID, "acme/security", "reviewer", domainartifact.KindSkill, "", "", "", "", nil, now.Add(3*time.Minute))
		if err := repository.Create(ctx, artifactA); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, artifactB); err != nil {
			t.Fatal(err)
		}
		duplicate, _ := domainartifact.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000092"), namespaceAID, "acme/security", "reviewer", domainartifact.KindSkill, "", "", "", "", nil, now)
		if err := repository.Create(ctx, duplicate); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate identity class = %q: %v", typedClass(err), err)
		}
		wrongNamespace, _ := domainartifact.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000093"), namespaceAID, "acme/other", "tool", domainartifact.KindBundle, "", "", "", "", nil, now)
		if err := repository.Create(ctx, wrongNamespace); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("mismatched namespace class = %q: %v", typedClass(err), err)
		}
		identity, _ := shared.ParseArtifactReference("acme/security/reviewer")
		got, err := repository.GetByIdentity(ctx, tenantA, identity)
		if err != nil || got.ID() != artifactAID || got.Kind() != domainartifact.KindSkill || got.Labels()["security/tier"] != "reviewed" {
			t.Fatalf("get: %#v %v", got, err)
		}
		values, err := repository.List(ctx, tenantA, nil, 50)
		if err != nil || len(values) != 1 || values[0].TenantID() != tenantA {
			t.Fatalf("tenant list: %#v %v", values, err)
		}
		if _, err := repository.Get(ctx, tenantA, artifactBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.artifacts SET kind = 'bundle'
 WHERE tenant_id = $1::uuid AND artifact_id = $2::uuid`, tenantA.String(), artifactAID.String()); err == nil {
			t.Fatal("database allowed Artifact kind mutation")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.artifacts SET labels = '{"UPPER":"invalid"}'::jsonb
 WHERE tenant_id = $1::uuid AND artifact_id = $2::uuid`, tenantA.String(), artifactAID.String()); err == nil {
			t.Fatal("database accepted invalid Artifact labels")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifacts
 WHERE tenant_id = $1::uuid AND artifact_id = $2::uuid`, tenantA.String(), artifactAID.String()); err == nil {
			t.Fatal("database allowed Artifact deletion")
		}
	})
	t.Run("artifact_version_repository", func(t *testing.T) {
		conn := newDB(t, "artifact_version_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		parse := func(value string) shared.UUID {
			parsed, err := shared.ParseUUID(value)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
		parseDigest := func(character string) shared.Digest {
			parsed, err := shared.ParseDigest("sha256:" + strings.Repeat(character, 64))
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
		tenantA := parse("0198fc21-ced5-7000-8000-000000000100")
		tenantB := parse("0198fc21-ced5-7000-8000-000000000101")
		publisherAID := parse("0198fc21-ced5-7000-8000-000000000110")
		publisherBID := parse("0198fc21-ced5-7000-8000-000000000111")
		namespaceAID := parse("0198fc21-ced5-7000-8000-000000000120")
		namespaceBID := parse("0198fc21-ced5-7000-8000-000000000121")
		artifactAID := parse("0198fc21-ced5-7000-8000-000000000130")
		artifactBID := parse("0198fc21-ced5-7000-8000-000000000131")
		versionAID := parse("0198fc21-ced5-7000-8000-000000000140")
		versionBID := parse("0198fc21-ced5-7000-8000-000000000141")
		versionCID := parse("0198fc21-ced5-7000-8000-000000000148")
		remoteArtifactID := parse("0198fc21-ced5-7000-8000-000000000149")
		remoteVersionID := parse("0198fc21-ced5-7000-8000-000000000150")
		importVersionID := parse("0198fc21-ced5-7000-8000-000000000151")
		importRecordID := parse("0198fc21-ced5-7000-8000-000000000152")
		now := time.Date(2026, 9, 5, 13, 0, 0, 0, time.UTC)
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantA, tenantB))
		execSQL("CREATE ROLE db005_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT ON public.tenants TO db005_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.publishers TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.publisher_state_records TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.namespaces TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifacts TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifact_versions TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifact_sources TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifact_descriptors TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifact_requirements TO db005_service")
		execSQL("GRANT SELECT, INSERT ON public.artifact_dependencies TO db005_service")
		execSQL("SET ROLE db005_service")

		publisherRepository, _ := postgrespublisher.NewRepository(conn)
		for _, value := range []domainpublisher.Publisher{
			mustPublisher(t, tenantA, publisherAID, "acme", now),
			mustPublisher(t, tenantB, publisherBID, "acme", now),
		} {
			if err := publisherRepository.Create(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
		reason, _ := shared.NewReasonCode("ownership.confirmed")
		if _, err := publisherRepository.ChangeState(ctx, tenantA, publisherAID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, tenantB, publisherBID, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		namespaceRepository, _ := postgresnamespace.NewRepository(conn)
		for _, value := range []domainnamespace.Namespace{
			mustNamespace(t, tenantA, namespaceAID, publisherAID, now),
			mustNamespace(t, tenantB, namespaceBID, publisherBID, now),
		} {
			if err := namespaceRepository.Create(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
		artifactRepository, _ := postgresartifact.NewRepository(conn)
		for _, value := range []domainartifact.Artifact{
			mustArtifact(t, tenantA, artifactAID, namespaceAID, now),
			mustArtifact(t, tenantB, artifactBID, namespaceBID, now),
		} {
			if err := artifactRepository.Create(ctx, value); err != nil {
				t.Fatal(err)
			}
		}

		repository, _ := postgresartifactversion.NewRepository(conn)
		semantic, _ := domainartifactversion.ParseSemanticVersion("1.2.3-rc.1+linux.amd64")
		versionA, _ := domainartifactversion.New(tenantA, versionAID, artifactAID, publisherAID, semantic, parseDigest("a"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now.Add(2*time.Minute))
		versionB, _ := domainartifactversion.New(tenantB, versionBID, artifactBID, publisherBID, semantic, parseDigest("a"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now.Add(2*time.Minute))
		if err := repository.Create(ctx, versionA); err != nil {
			t.Fatal(err)
		}
		if err := repository.Create(ctx, versionB); err != nil {
			t.Fatal(err)
		}
		conflictingVersion, _ := domainartifactversion.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000142"), artifactAID, publisherAID, semantic, parseDigest("b"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now)
		if err := repository.Create(ctx, conflictingVersion); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("semantic-version conflict class = %q: %v", typedClass(err), err)
		}
		otherSemantic, _ := domainartifactversion.ParseSemanticVersion("1.2.4")
		conflictingDigest, _ := domainartifactversion.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000143"), artifactAID, publisherAID, otherSemantic, parseDigest("a"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now)
		if err := repository.Create(ctx, conflictingDigest); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("digest conflict class = %q: %v", typedClass(err), err)
		}
		crossTenantPublisher, _ := domainartifactversion.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000144"), artifactAID, publisherBID, otherSemantic, parseDigest("c"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now)
		if err := repository.Create(ctx, crossTenantPublisher); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant publisher class = %q: %v", typedClass(err), err)
		}
		got, err := repository.GetBySemanticVersion(ctx, tenantA, artifactAID, semantic)
		if err != nil || got.ID() != versionAID || got.Digest() != parseDigest("a") || got.Lifecycle() != domainartifactversion.LifecycleActive {
			t.Fatalf("get by version: %#v %v", got, err)
		}
		got, err = repository.GetByDigest(ctx, tenantA, parseDigest("a"))
		if err != nil || got.ID() != versionAID {
			t.Fatalf("get by digest: %#v %v", got, err)
		}
		values, err := repository.List(ctx, tenantA, artifactAID, nil, 50)
		if err != nil || len(values) != 1 || values[0].TenantID() != tenantA {
			t.Fatalf("tenant list: %#v %v", values, err)
		}
		if _, err := repository.Get(ctx, tenantA, versionBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}
		semanticC, _ := domainartifactversion.ParseSemanticVersion("1.2.5")
		versionC, _ := domainartifactversion.New(tenantA, versionCID, artifactAID, publisherAID, semanticC, parseDigest("d"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryOCI, now.Add(3*time.Minute))
		if err := repository.Create(ctx, versionC); err != nil {
			t.Fatal(err)
		}
		remoteArtifact, _ := domainartifact.New(tenantA, remoteArtifactID, namespaceAID, "acme/security", "remote-server", domainartifact.KindMCPServer, "", "", "", "", nil, now.Add(3*time.Minute))
		if err := artifactRepository.Create(ctx, remoteArtifact); err != nil {
			t.Fatal(err)
		}
		remoteSemantic, _ := domainartifactversion.ParseSemanticVersion("1.0.0")
		remoteVersion, _ := domainartifactversion.New(tenantA, remoteVersionID, remoteArtifactID, publisherAID, remoteSemantic, parseDigest("e"), domainartifact.KindMCPServer, domainartifactversion.ClassRemoteService, domainartifactversion.DeliveryRemote, now.Add(4*time.Minute))
		if err := repository.Create(ctx, remoteVersion); err != nil {
			t.Fatal(err)
		}
		importSemantic, _ := domainartifactversion.ParseSemanticVersion("1.2.6")
		importVersion, _ := domainartifactversion.New(tenantA, importVersionID, artifactAID, publisherAID, importSemantic, parseDigest("f"), domainartifact.KindSkill, domainartifactversion.ClassInstructional, domainartifactversion.DeliveryImportedSource, now.Add(5*time.Minute))
		if err := repository.Create(ctx, importVersion); err != nil {
			t.Fatal(err)
		}

		sourceRepository, err := postgresartifactsource.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		resolvedA := "registry.example/acme/security/reviewer@" + versionA.Digest().String()
		resolvedB := "registry.example/acme/security/reviewer@" + versionB.Digest().String()
		sourceA, _ := domainartifactsource.NewOCI(tenantA, versionAID, "registry.example/acme/security/reviewer:1.2.3-rc.1", resolvedA, versionA.Digest())
		sourceB, _ := domainartifactsource.NewOCI(tenantB, versionBID, "registry.example/acme/security/reviewer:1.2.3-rc.1", resolvedB, versionB.Digest())
		if err := sourceRepository.Create(ctx, sourceA); err != nil {
			t.Fatal(err)
		}
		if err := sourceRepository.Create(ctx, sourceB); err != nil {
			t.Fatal(err)
		}
		remoteSource, _ := domainartifactsource.NewRemote(tenantA, remoteVersionID, "https://agents.example/card.json", "https://objects.example/cards/immutable", remoteVersion.Digest(), "https://agents.example/a2a")
		if err := sourceRepository.Create(ctx, remoteSource); err != nil {
			t.Fatal(err)
		}
		importSource, _ := domainartifactsource.NewImportRecord(tenantA, importVersionID, importRecordID, "mcp-registry:server/example@1.0.0", importVersion.Digest())
		if err := sourceRepository.Create(ctx, importSource); err != nil {
			t.Fatal(err)
		}
		if err := sourceRepository.Create(ctx, sourceA); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate source class = %q: %v", typedClass(err), err)
		}
		mismatchedDigest, _ := domainartifactsource.NewOCI(tenantA, versionAID, "registry.example/acme/security/reviewer:other", "registry.example/acme/security/reviewer@"+parseDigest("b").String(), parseDigest("b"))
		if err := sourceRepository.Create(ctx, mismatchedDigest); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("source digest mismatch class = %q: %v", typedClass(err), err)
		}
		gotSource, err := sourceRepository.Get(ctx, tenantA, versionAID)
		if err != nil || gotSource.Kind() != domainartifactsource.KindOCI || gotSource.SubmittedReference() == gotSource.ResolvedReference() || gotSource.ResolvedDigest() != versionA.Digest() {
			t.Fatalf("get source: %#v %v", gotSource, err)
		}
		if _, err := sourceRepository.Get(ctx, tenantA, versionBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant source get class = %q: %v", typedClass(err), err)
		}
		gotRemote, err := sourceRepository.Get(ctx, tenantA, remoteVersionID)
		if err != nil || gotRemote.Kind() != domainartifactsource.KindRemote || gotRemote.Endpoint() != "https://agents.example/a2a" {
			t.Fatalf("get remote source: %#v %v", gotRemote, err)
		}
		gotImport, err := sourceRepository.Get(ctx, tenantA, importVersionID)
		gotImportID, hasImportID := gotImport.ImportRecordID()
		if err != nil || gotImport.Kind() != domainartifactsource.KindImportRecord || !hasImportID || gotImportID != importRecordID {
			t.Fatalf("get import source: %#v %v", gotImport, err)
		}

		descriptorRepository, err := postgresartifactdescriptor.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		requirementMetadata := []byte(`{"schema_version":1,"capabilities":{"required":["scm.repository.read"]},"network":{"profile":"thinkpixel-only"}}`)
		dependencyMetadataA := []byte(`{"schema_version":1,"name":"review-engine","artifact":"acme/platform/review-engine","required":true,"catalog":"production","selector":{"digest":"sha256:` + strings.Repeat("1", 64) + `"}}`)
		dependencyMetadataB := []byte(`{"schema_version":1,"name":"optional-linter","artifact":"acme/platform/linter","required":false,"source":"enterprise","selector":{"range":">=1.2.0 <2.0.0"}}`)
		dependencyMetadataC := []byte(`{"schema_version":1,"name":"schema-tools","artifact":"acme/platform/schema-tools","required":true,"selector":{"version":"1.4.2"}}`)
		dependenciesMetadata := `[` + string(dependencyMetadataA) + `,` + string(dependencyMetadataB) + `,` + string(dependencyMetadataC) + `]`
		metadataA := []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme/security","name":"reviewer","version":"1.2.3-rc.1+linux.amd64"},"requirements":` + string(requirementMetadata) + `,"dependencies":` + dependenciesMetadata + `,"spec":{}}`)
		descriptorA, err := domainartifactdescriptor.New(tenantA, versionAID, shared.SHA256Digest(metadataA), "application/vnd.thinkpixel.skill.manifest.v1+json", metadataA)
		if err != nil {
			t.Fatal(err)
		}
		metadataB := append([]byte(nil), metadataA...)
		descriptorB, err := domainartifactdescriptor.New(tenantB, versionBID, shared.SHA256Digest(metadataB), "application/vnd.thinkpixel.skill.manifest.v1+json", metadataB)
		if err != nil {
			t.Fatal(err)
		}
		if err := descriptorRepository.Create(ctx, descriptorA); err != nil {
			t.Fatal(err)
		}
		if err := descriptorRepository.Create(ctx, descriptorB); err != nil {
			t.Fatal(err)
		}
		emptyRequirementMetadata := []byte(`{"schema_version":1}`)
		importDescriptorMetadata := []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme/security","name":"reviewer","version":"1.2.6"},"requirements":` + string(emptyRequirementMetadata) + `,"dependencies":[],"spec":{}}`)
		importDescriptor, err := domainartifactdescriptor.New(tenantA, importVersionID, shared.SHA256Digest(importDescriptorMetadata), "application/vnd.thinkpixel.skill.manifest.v1+json", importDescriptorMetadata)
		if err != nil {
			t.Fatal(err)
		}
		if err := descriptorRepository.Create(ctx, importDescriptor); err != nil {
			t.Fatal(err)
		}
		if err := descriptorRepository.Create(ctx, descriptorA); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate descriptor class = %q: %v", typedClass(err), err)
		}
		wrongCoordinates := []byte(`{"schema_version":1,"kind":"skill","artifact":{"namespace":"acme/security","name":"reviewer","version":"1.2.4"},"requirements":{"schema_version":1},"dependencies":[],"spec":{}}`)
		mismatchedDescriptor, err := domainartifactdescriptor.New(tenantA, versionAID, shared.SHA256Digest(wrongCoordinates), "application/vnd.thinkpixel.skill.manifest.v1+json", wrongCoordinates)
		if err != nil {
			t.Fatal(err)
		}
		if err := descriptorRepository.Create(ctx, mismatchedDescriptor); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("descriptor coordinate mismatch class = %q: %v", typedClass(err), err)
		}
		gotDescriptor, err := descriptorRepository.Get(ctx, tenantA, versionAID)
		if err != nil || gotDescriptor.DescriptorDigest() != descriptorA.DescriptorDigest() || string(gotDescriptor.NormalizedMetadata()) != string(metadataA) {
			t.Fatalf("get descriptor: %#v %v", gotDescriptor, err)
		}
		if _, err := descriptorRepository.Get(ctx, tenantA, versionBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant descriptor get class = %q: %v", typedClass(err), err)
		}

		requirementRepository, err := postgresartifactrequirement.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		requirementA, err := domainartifactrequirement.New(tenantA, versionAID, shared.SHA256Digest(requirementMetadata), requirementMetadata)
		if err != nil {
			t.Fatal(err)
		}
		requirementB, err := domainartifactrequirement.New(tenantB, versionBID, shared.SHA256Digest(requirementMetadata), requirementMetadata)
		if err != nil {
			t.Fatal(err)
		}
		if err := requirementRepository.Create(ctx, requirementA); err != nil {
			t.Fatal(err)
		}
		if err := requirementRepository.Create(ctx, requirementB); err != nil {
			t.Fatal(err)
		}
		if err := requirementRepository.Create(ctx, requirementA); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate requirement class = %q: %v", typedClass(err), err)
		}
		mismatchedRequirementMetadata := []byte(`{"schema_version":1,"network":{"profile":"none"}}`)
		mismatchedRequirement, err := domainartifactrequirement.New(tenantA, versionAID, shared.SHA256Digest(mismatchedRequirementMetadata), mismatchedRequirementMetadata)
		if err != nil {
			t.Fatal(err)
		}
		if err := requirementRepository.Create(ctx, mismatchedRequirement); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("descriptor requirement mismatch class = %q: %v", typedClass(err), err)
		}
		gotRequirement, err := requirementRepository.Get(ctx, tenantA, versionAID)
		if err != nil || gotRequirement.RequirementDigest() != requirementA.RequirementDigest() || string(gotRequirement.NormalizedRequirement()) != string(requirementMetadata) {
			t.Fatalf("get requirement: %#v %v", gotRequirement, err)
		}
		if _, err := requirementRepository.Get(ctx, tenantA, versionBID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant requirement get class = %q: %v", typedClass(err), err)
		}

		dependencyRepository, err := postgresartifactdependency.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		dependencyA0, err := domainartifactdependency.New(tenantA, versionAID, 0, shared.SHA256Digest(dependencyMetadataA), dependencyMetadataA)
		if err != nil {
			t.Fatal(err)
		}
		dependencyA1, err := domainartifactdependency.New(tenantA, versionAID, 1, shared.SHA256Digest(dependencyMetadataB), dependencyMetadataB)
		if err != nil {
			t.Fatal(err)
		}
		dependencyB0, err := domainartifactdependency.New(tenantB, versionBID, 0, shared.SHA256Digest(dependencyMetadataA), dependencyMetadataA)
		if err != nil {
			t.Fatal(err)
		}
		dependencyA2, err := domainartifactdependency.New(tenantA, versionAID, 2, shared.SHA256Digest(dependencyMetadataC), dependencyMetadataC)
		if err != nil {
			t.Fatal(err)
		}
		for _, dependency := range []domainartifactdependency.ArtifactDependency{dependencyA0, dependencyA1, dependencyA2, dependencyB0} {
			if err := dependencyRepository.Create(ctx, dependency); err != nil {
				t.Fatal(err)
			}
		}
		if err := dependencyRepository.Create(ctx, dependencyA0); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("duplicate dependency class = %q: %v", typedClass(err), err)
		}
		mismatchedDependency, err := domainartifactdependency.New(tenantA, importVersionID, 0, shared.SHA256Digest(dependencyMetadataA), dependencyMetadataA)
		if err != nil {
			t.Fatal(err)
		}
		if err := dependencyRepository.Create(ctx, mismatchedDependency); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("descriptor dependency mismatch class = %q: %v", typedClass(err), err)
		}
		gotDependency, err := dependencyRepository.Get(ctx, tenantA, versionAID, 0)
		if err != nil || gotDependency.DependencyDigest() != dependencyA0.DependencyDigest() || string(gotDependency.NormalizedDependency()) != string(dependencyMetadataA) {
			t.Fatalf("get dependency: %#v %v", gotDependency, err)
		}
		dependencies, err := dependencyRepository.List(ctx, tenantA, versionAID)
		if err != nil || len(dependencies) != 3 || dependencies[0].Index() != 0 || dependencies[1].Index() != 1 || dependencies[1].Name() != "optional-linter" || dependencies[2].SelectorKind() != domainartifactdependency.SelectorVersion {
			t.Fatalf("list dependencies: %#v %v", dependencies, err)
		}
		if _, err := dependencyRepository.Get(ctx, tenantA, versionBID, 0); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant dependency get class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_dependencies
  (tenant_id, artifact_version_id, dependency_index, schema_version, dependency_name,
   artifact_identity, required, catalog, selector_kind, selector_value, dependency_digest,
   normalized_bytes, normalized_dependency)
 VALUES ($1::uuid, $2::uuid, 3, 1, 'review-engine', 'acme/platform/review-engine', true,
         'production', 'digest', $3, $4, convert_to($5, 'UTF8'), $5::jsonb)`,
			tenantA.String(), versionAID.String(), parseDigest("1").String(), shared.SHA256Digest(dependencyMetadataA).String(), string(dependencyMetadataA)); err == nil {
			t.Fatal("database accepted a dependency at a descriptor-mismatched index")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.artifact_dependencies SET dependency_name = 'rewritten'
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid AND dependency_index = 0`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactDependency mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifact_dependencies
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid AND dependency_index = 0`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactDependency deletion")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_requirements
  (tenant_id, artifact_version_id, schema_version, requirement_digest, normalized_bytes, normalized_requirement)
 VALUES ($1::uuid, $2::uuid, 1, $3, convert_to($4, 'UTF8'), $4::jsonb)`,
			tenantA.String(), importVersionID.String(), shared.SHA256Digest(mismatchedRequirementMetadata).String(), string(mismatchedRequirementMetadata)); err == nil {
			t.Fatal("database accepted a requirement that differs from its parent descriptor")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.artifact_requirements SET normalized_requirement = '{"schema_version":1}'::jsonb
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactRequirement mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifact_requirements
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactRequirement deletion")
		}
		if _, err := conn.Exec(ctx, `WITH metadata(value) AS (
    SELECT jsonb_build_object(
        'schema_version', 1,
        'kind', 'skill',
        'artifact', jsonb_build_object('namespace', 'acme/security', 'name', 'reviewer', 'version', '1.2.5'),
        'requirements', '{}'::jsonb,
        'dependencies', '[]'::jsonb,
        'spec', jsonb_build_object('value', repeat('x', 1048576))
    )
)
INSERT INTO public.artifact_descriptors
  (tenant_id, artifact_version_id, artifact_id, namespace_id, schema_version, kind, namespace_path,
   artifact_name, semantic_version, media_type, descriptor_digest, normalized_bytes, normalized_metadata)
SELECT $1::uuid, $2::uuid, $3::uuid, $4::uuid, 1, 'skill', 'acme/security', 'reviewer', '1.2.5',
       'application/vnd.thinkpixel.skill.manifest.v1+json', $5, convert_to(value::text, 'UTF8'), value
  FROM metadata`, tenantA.String(), versionCID.String(), artifactAID.String(), namespaceAID.String(), parseDigest("0").String()); err == nil {
			t.Fatal("database accepted oversized normalized descriptor metadata")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.artifact_descriptors SET normalized_metadata = '{"rewritten":true}'::jsonb
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactDescriptor mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifact_descriptors
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactDescriptor deletion")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_sources
  (tenant_id, artifact_version_id, kind, submitted_reference, resolved_reference, resolved_digest, delivery_model)
 VALUES ($1::uuid, $2::uuid, 'oci', 'registry.example/acme/security/reviewer:latest',
         'registry.example/acme/security/reviewer:latest', $3, 'oci')`, tenantA.String(), versionCID.String(), versionC.Digest().String()); err == nil {
			t.Fatal("database accepted a mutable OCI resolved reference")
		}
		if _, err := conn.Exec(ctx, `UPDATE public.artifact_sources SET submitted_reference = 'rewritten'
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactSource mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifact_sources
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactSource deletion")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_versions
  (tenant_id, artifact_version_id, artifact_id, publisher_id, semantic_version, resolved_digest, kind, artifact_class, delivery_model, registered_at)
 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'v1.2.5', $5, 'skill', 'instructional', 'oci', $6)`, tenantA.String(), parse("0198fc21-ced5-7000-8000-000000000145").String(), artifactAID.String(), publisherAID.String(), parseDigest("d").String(), now); err == nil {
			t.Fatal("database accepted invalid semantic version")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_versions
  (tenant_id, artifact_version_id, artifact_id, publisher_id, semantic_version, resolved_digest, kind, artifact_class, delivery_model, registered_at)
 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '1.2.5', $5, 'skill', 'remote-service', 'remote', $6)`, tenantA.String(), parse("0198fc21-ced5-7000-8000-000000000146").String(), artifactAID.String(), publisherAID.String(), parseDigest("e").String(), now); err == nil {
			t.Fatal("database accepted invalid taxonomy combination")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.artifact_versions
  (tenant_id, artifact_version_id, artifact_id, publisher_id, semantic_version, resolved_digest, kind, artifact_class, delivery_model, registered_at)
 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '1.2.6', $5, 'skill', 'instructional', 'oci', $6)`, tenantA.String(), parse("0198fc21-ced5-7000-8000-000000000147").String(), artifactAID.String(), publisherAID.String(), "SHA256:"+strings.Repeat("f", 64), now); err == nil {
			t.Fatal("database accepted invalid digest")
		}
		for name, statement := range map[string]string{
			"digest":           `UPDATE public.artifact_versions SET resolved_digest = 'sha256:` + strings.Repeat("b", 64) + `' WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`,
			"semantic version": `UPDATE public.artifact_versions SET semantic_version = '2.0.0' WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`,
			"payload metadata": `UPDATE public.artifact_versions SET artifact_class = 'executable-local' WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`,
			"lifecycle":        `UPDATE public.artifact_versions SET lifecycle = 'deprecated' WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`,
			"registration time": `UPDATE public.artifact_versions SET registered_at = registered_at + interval '1 second'
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`,
		} {
			if _, err := conn.Exec(ctx, statement, tenantA.String(), versionAID.String()); err == nil {
				t.Fatalf("database allowed ArtifactVersion %s mutation", name)
			}
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.artifact_versions
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()); err == nil {
			t.Fatal("database allowed ArtifactVersion deletion")
		}
		var preservedDigest, preservedSemanticVersion, preservedLifecycle string
		if err := conn.QueryRow(ctx, `SELECT resolved_digest, semantic_version, lifecycle
  FROM public.artifact_versions
 WHERE tenant_id = $1::uuid AND artifact_version_id = $2::uuid`, tenantA.String(), versionAID.String()).Scan(&preservedDigest, &preservedSemanticVersion, &preservedLifecycle); err != nil {
			t.Fatal(err)
		}
		if preservedDigest != versionA.Digest().String() || preservedSemanticVersion != semantic.String() || preservedLifecycle != "active" {
			t.Fatal("failed mutation changed the registered ArtifactVersion")
		}
	})
	t.Run("command", func(t *testing.T) {
		_ = newDB(t, "command_test")
		for i, action := range []string{"status", "up", "status", "up"} {
			command := exec.CommandContext(ctx, "go", "run", "../../../../cmd/migrate", action)
			command.Env = append(os.Environ(), "TPMP_MIGRATION_DATABASE_URL_REF=env:DB001_TEST_URL", "DB001_TEST_URL=postgres://postgres@"+address+"/command_test?sslmode=disable")
			out, err := command.CombinedOutput()
			want := "applied"
			if i == 0 {
				want = "pending"
			}
			expected := "000001_tenants.sql " + want + "\n000002_publishers.sql " + want + "\n000003_namespaces.sql " + want + "\n000004_artifacts.sql " + want + "\n000005_artifact_versions.sql " + want + "\n000006_artifact_version_mutation_guards.sql " + want + "\n000007_artifact_sources.sql " + want + "\n000008_artifact_descriptors.sql " + want + "\n000009_artifact_requirements.sql " + want + "\n000010_artifact_dependencies.sql " + want
			if err != nil || strings.TrimSpace(string(out)) != expected {
				t.Fatalf("command %s: %s %v", action, out, err)
			}
		}
	})
	t.Run("history_gap", func(t *testing.T) {
		conn := newDB(t, "gap_test")
		files := fstest.MapFS{"000001_a.sql": {Data: []byte("SELECT 1")}, "000002_b.sql": {Data: []byte("SELECT 2")}}
		if _, err := Run(ctx, conn, files, true); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "DELETE FROM public.schema_migrations WHERE name = '000001_a.sql'"); err != nil {
			t.Fatal(err)
		}
		for _, apply := range []bool{false, true} {
			if _, err := Run(ctx, conn, files, apply); err == nil {
				t.Fatal("accepted history gap")
			}
		}
	})
	t.Run("rollback", func(t *testing.T) {
		conn := newDB(t, "rollback_test")
		files := fstest.MapFS{"000001_ok.sql": {Data: []byte("CREATE TABLE public.rollback_probe (id int)")}, "000002_fail.sql": {Data: []byte("SELECT missing_column")}}
		if _, err := Run(ctx, conn, files, true); err == nil {
			t.Fatal("accepted failed migration")
		}
		var clean bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.rollback_probe') IS NULL AND to_regclass('public.schema_migrations') IS NULL").Scan(&clean); err != nil || !clean {
			t.Fatal("migration did not roll back")
		}
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "INSERT INTO public.schema_migrations (name, checksum) VALUES ('000011_unknown.sql', repeat('0',64))"); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(ctx, conn, migrations.Files, true); err == nil {
			t.Fatal("accepted unknown migration")
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		conn := newDB(t, "concurrent_test")
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				peer, err := connect("concurrent_test")
				if err != nil {
					t.Error(err)
					return
				}
				defer func() { _ = peer.Close(context.Background()) }()
				if _, err := Run(ctx, peer, migrations.Files, true); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&n); err != nil || n != 10 {
			t.Fatalf("concurrent ledger: %d %v", n, err)
		}
	})
}

func mustPublisher(t *testing.T, tenantID, publisherID shared.UUID, slug string, at time.Time) domainpublisher.Publisher {
	t.Helper()
	value, err := domainpublisher.New(tenantID, publisherID, slug, "Publisher", "", at)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustNamespace(t *testing.T, tenantID, namespaceID, publisherID shared.UUID, at time.Time) domainnamespace.Namespace {
	t.Helper()
	value, err := domainnamespace.New(tenantID, namespaceID, "acme/security", publisherID, at)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustArtifact(t *testing.T, tenantID, artifactID, namespaceID shared.UUID, at time.Time) domainartifact.Artifact {
	t.Helper()
	value, err := domainartifact.New(tenantID, artifactID, namespaceID, "acme/security", "reviewer", domainartifact.KindSkill, "", "", "", "", nil, at)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func typedClass(err error) shared.ErrorClass {
	var typed *shared.TypedError
	if errors.As(err, &typed) {
		return typed.Class()
	}
	return ""
}
