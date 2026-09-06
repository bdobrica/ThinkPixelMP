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

	postgres "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres"
	postgresartifact "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifact"
	postgresartifactdependency "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactdependency"
	postgresartifactdescriptor "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactdescriptor"
	postgresartifactrequirement "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactrequirement"
	postgresartifactsource "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactsource"
	postgresartifactversion "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/artifactversion"
	postgresaudit "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/audit"
	postgresidempotency "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/idempotency"
	postgresnamespace "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/namespace"
	postgresoutbox "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/outbox"
	postgrespublisher "github.com/bdobrica/ThinkPixelMP/internal/adapters/postgres/publisher"
	domainartifact "github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	domainartifactdependency "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdependency"
	domainartifactdescriptor "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactdescriptor"
	domainartifactrequirement "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactrequirement"
	domainartifactsource "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactsource"
	domainartifactversion "github.com/bdobrica/ThinkPixelMP/internal/domain/artifactversion"
	domainaudit "github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	domainidempotency "github.com/bdobrica/ThinkPixelMP/internal/domain/idempotency"
	domainnamespace "github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	domainoutbox "github.com/bdobrica/ThinkPixelMP/internal/domain/outbox"
	domainpublisher "github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	requestID, err := shared.ParseUUID("0198fc21-ced5-7000-8000-000000000009")
	if err != nil {
		t.Fatal(err)
	}
	auditActor, err := domainaudit.NewActor("integration:test-principal", &requestID, strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx = domainaudit.WithActor(ctx, auditActor)
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
		if err != nil || len(states) != 14 || states[0].Applied || states[1].Applied || states[2].Applied || states[3].Applied || states[4].Applied || states[5].Applied || states[6].Applied || states[7].Applied || states[8].Applied || states[9].Applied || states[10].Applied || states[11].Applied || states[12].Applied || states[13].Applied {
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
		if err != nil || !states[0].Applied || !states[1].Applied || !states[2].Applied || !states[3].Applied || !states[4].Applied || !states[5].Applied || !states[6].Applied || !states[7].Applied || !states[8].Applied || !states[9].Applied || !states[10].Applied || !states[11].Applied || !states[12].Applied || !states[13].Applied {
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
	t.Run("transaction_manager", func(t *testing.T) {
		conn := newDB(t, "transaction_test")
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
			t.Helper()
			parsed, err := shared.ParseUUID(value)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}

		tenantID := parse("0198fc21-ced5-7000-8000-000000000002")
		otherTenantID := parse("0198fc21-ced5-7000-8000-000000000003")
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantID.String(), otherTenantID.String()))
		execSQL("CREATE ROLE db014_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT ON public.tenants TO db014_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.publishers TO db014_service")
		execSQL("GRANT SELECT, INSERT ON public.publisher_state_records TO db014_service")
		execSQL("GRANT SELECT, INSERT ON public.audit_events TO db014_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.idempotency_records TO db014_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.tenant_event_sequences TO db014_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.outbox_messages TO db014_service")
		execSQL("SET ROLE db014_service")

		manager, err := postgres.NewTransactionManager(conn)
		if err != nil {
			t.Fatal(err)
		}
		publisherRepository, _ := postgrespublisher.NewRepository(conn)
		idempotencyRepository, _ := postgresidempotency.NewRepository(conn)
		auditRepository, _ := postgresaudit.NewRepository(conn)
		outboxRepository, _ := postgresoutbox.NewRepository(conn)
		now := time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)
		digest, _ := shared.ParseDigest("sha256:" + strings.Repeat("d", 64))
		action, _ := shared.NewReasonCode("publisher.create")
		publisherID := parse("0198fc21-ced5-7000-8000-000000000004")
		recordID := parse("0198fc21-ced5-7000-8000-000000000005")
		publisher, _ := domainpublisher.New(tenantID, publisherID, "atomic-rollback", "", "", now)
		record, _ := domainidempotency.New(tenantID, recordID, "integration:test-principal", action, "rollback-key", digest, now, now.Add(24*time.Hour))
		if _, err := outboxRepository.NextSequence(ctx, tenantID); typedClass(err) != shared.ErrorInvalid {
			t.Fatalf("outbox sequence outside transaction class = %q: %v", typedClass(err), err)
		}
		rollbackMessageID := parse("0198fc21-ced5-7000-8000-000000000008")
		message := func(id shared.UUID, sequence uint64) domainoutbox.Message {
			t.Helper()
			payload := []byte(fmt.Sprintf(`{"specversion":"1.0","id":"%s","source":"urn:thinkpixel:mp:integration","type":"io.thinkpixel.mp.artifact.registered.v1","subject":"%s","time":"%s","datacontenttype":"%s","sequence":%d,"data":{"tenant_id":"%s","transaction_cursor":"cursor-%d","artifact_version_id":"%s","artifact_digest":"sha256:%s","descriptor_digest":"sha256:%s"}}`,
				id.String(), id.String(), now.Format(time.RFC3339Nano), domainoutbox.DataContentType, sequence,
				tenantID.String(), sequence, id.String(), strings.Repeat("a", 64), strings.Repeat("b", 64)))
			value, err := domainoutbox.New(tenantID, id, sequence, "urn:thinkpixel:mp:integration", "io.thinkpixel.mp.artifact.registered.v1", id.String(), payload, now)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		forcedRollback := errors.New("forced rollback")

		err = manager.WithinTransaction(ctx, tenantID, func(transactionCtx context.Context) error {
			if err := publisherRepository.Create(transactionCtx, publisher); err != nil {
				return err
			}
			if _, created, err := idempotencyRepository.Acquire(transactionCtx, record); err != nil || !created {
				return fmt.Errorf("acquire idempotency record: created=%t: %w", created, err)
			}
			sequence, err := outboxRepository.NextSequence(transactionCtx, tenantID)
			if err != nil {
				return err
			}
			if err := outboxRepository.Record(transactionCtx, message(rollbackMessageID, sequence)); err != nil {
				return err
			}
			return forcedRollback
		})
		if !errors.Is(err, forcedRollback) {
			t.Fatalf("callback error was not preserved: %v", err)
		}
		if _, err := publisherRepository.Get(ctx, tenantID, publisherID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("publisher survived rollback: %v", err)
		}
		if _, err := idempotencyRepository.Get(ctx, tenantID, record.Principal(), action, record.Key()); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("idempotency record survived rollback: %v", err)
		}
		if events, err := auditRepository.List(ctx, tenantID, nil, 20); err != nil || len(events) != 0 {
			t.Fatalf("audit survived rollback: %#v %v", events, err)
		}
		if _, err := outboxRepository.Get(ctx, tenantID, rollbackMessageID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("outbox message survived rollback: %v", err)
		}

		committedPublisherID := parse("0198fc21-ced5-7000-8000-000000000006")
		committedRecordID := parse("0198fc21-ced5-7000-8000-000000000007")
		committedPublisher, _ := domainpublisher.New(tenantID, committedPublisherID, "atomic-commit", "", "", now)
		committedRecord, _ := domainidempotency.New(tenantID, committedRecordID, "integration:test-principal", action, "commit-key", digest, now, now.Add(24*time.Hour))
		committedMessageID := parse("0198fc21-ced5-7000-8000-00000000000a")
		err = manager.WithinTransaction(ctx, tenantID, func(transactionCtx context.Context) error {
			if err := publisherRepository.Create(transactionCtx, committedPublisher); err != nil {
				return err
			}
			if err := manager.WithinTransaction(transactionCtx, tenantID, func(nestedCtx context.Context) error {
				_, created, err := idempotencyRepository.Acquire(nestedCtx, committedRecord)
				if err == nil && !created {
					return errors.New("nested transaction did not create idempotency record")
				}
				return err
			}); err != nil {
				return err
			}
			if err := manager.WithinTransaction(transactionCtx, otherTenantID, func(context.Context) error { return nil }); typedClass(err) != shared.ErrorInvalid {
				return fmt.Errorf("cross-tenant nested transaction class = %q: %w", typedClass(err), err)
			}
			sequence, err := outboxRepository.NextSequence(transactionCtx, tenantID)
			if err != nil {
				return err
			}
			if sequence != 1 {
				return fmt.Errorf("rolled-back event sequence was retained: %d", sequence)
			}
			return outboxRepository.Record(transactionCtx, message(committedMessageID, sequence))
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.Get(ctx, tenantID, committedPublisherID); err != nil {
			t.Fatalf("committed publisher missing: %v", err)
		}
		if got, err := idempotencyRepository.Get(ctx, tenantID, committedRecord.Principal(), action, committedRecord.Key()); err != nil || got.ID() != committedRecordID {
			t.Fatalf("committed idempotency record: %#v %v", got, err)
		}
		if events, err := auditRepository.List(ctx, tenantID, nil, 20); err != nil || len(events) != 1 {
			t.Fatalf("committed audit events: %#v %v", events, err)
		}
		if got, err := outboxRepository.Get(ctx, tenantID, committedMessageID); err != nil || got.Sequence() != 1 {
			t.Fatalf("committed outbox message: %#v %v", got, err)
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
		execSQL("GRANT SELECT, INSERT ON public.audit_events TO db002_service")
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
		got, err = repository.ChangeState(ctx, a, publisherAID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute))
		if err != nil || got.State() != domainpublisher.StateVerified || got.StateVersion() != 2 {
			t.Fatalf("change state: %#v %v", got, err)
		}
		if _, err := repository.ChangeState(ctx, a, publisherAID, 1, domainpublisher.StateSuspended, reason, "stale", now.Add(2*time.Minute)); typedCode(err) != "publisher.stale_state_version" {
			t.Fatalf("stale version code = %q: %v", typedCode(err), err)
		}
		if _, err := repository.ChangeState(ctx, a, publisherAID, 0, domainpublisher.StateSuspended, reason, "invalid", now.Add(2*time.Minute)); typedClass(err) != shared.ErrorInvalid {
			t.Fatalf("invalid version class = %q: %v", typedClass(err), err)
		}
		if _, err := repository.ChangeState(ctx, a, publisherAID, 2, domainpublisher.StateClaimed, reason, "invalid", now.Add(2*time.Minute)); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("invalid transition class = %q: %v", typedClass(err), err)
		}

		concurrentID := parse("0198fc21-ced5-7000-8000-000000000024")
		concurrentPublisher, _ := domainpublisher.New(a, concurrentID, "concurrent", "Concurrent", "", now)
		if err := repository.Create(ctx, concurrentPublisher); err != nil {
			t.Fatal(err)
		}
		peer, err := connect("publisher_test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = peer.Close(context.Background()) }()
		if _, err := peer.Exec(ctx, "SET ROLE db002_service"); err != nil {
			t.Fatal(err)
		}
		peerRepository, err := postgrespublisher.NewRepository(peer)
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 2)
		var transitions sync.WaitGroup
		transitions.Go(func() {
			_, transitionErr := repository.ChangeState(ctx, a, concurrentID, 1, domainpublisher.StateVerified, reason, "verified", now.Add(3*time.Minute))
			results <- transitionErr
		})
		transitions.Go(func() {
			_, transitionErr := peerRepository.ChangeState(ctx, a, concurrentID, 1, domainpublisher.StateSuspended, reason, "suspended", now.Add(3*time.Minute))
			results <- transitionErr
		})
		transitions.Wait()
		close(results)
		var successful, stale int
		for transitionErr := range results {
			switch typedCode(transitionErr) {
			case "":
				successful++
			case "publisher.stale_state_version":
				stale++
			default:
				t.Fatalf("unexpected concurrent transition: %v", transitionErr)
			}
		}
		if successful != 1 || stale != 1 {
			t.Fatalf("concurrent outcomes: successful=%d stale=%d", successful, stale)
		}
		if current, err := repository.Get(ctx, a, concurrentID); err != nil || current.StateVersion() != 2 {
			t.Fatalf("concurrent publisher: %#v %v", current, err)
		}
		unauditedID := parse("0198fc21-ced5-7000-8000-000000000022")
		unaudited, _ := domainpublisher.New(a, unauditedID, "no-actor", "No actor", "", now)
		if err := repository.Create(context.Background(), unaudited); typedClass(err) != shared.ErrorUnauthorized {
			t.Fatalf("missing audit actor class = %q: %v", typedClass(err), err)
		}
		if _, err := repository.Get(ctx, a, unauditedID); typedClass(err) != shared.ErrorNotFound {
			t.Fatal("mutation survived failed audit recording")
		}
		auditRepository, err := postgresaudit.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		auditEvents, err := auditRepository.List(ctx, a, nil, 20)
		if err != nil || len(auditEvents) != 4 {
			t.Fatalf("publisher audits: %#v %v", auditEvents, err)
		}
		decision, hasDecision := auditEvents[1].Decision()
		if auditEvents[0].ActorID() != "integration:test-principal" || auditEvents[1].Action().String() != postgresaudit.ActionPublisherStateChanged || !hasDecision || decision.String() != "verified" {
			t.Fatalf("publisher audit content: %#v", auditEvents)
		}
		if _, err := auditRepository.Get(ctx, b, auditEvents[0].ID()); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant audit get class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.audit_events SET actor_id = 'rewritten' WHERE tenant_id = $1::uuid`, tenantA); err == nil {
			t.Fatal("database allowed AuditEvent mutation")
		}
		if _, err := conn.Exec(ctx, `DELETE FROM public.audit_events WHERE tenant_id = $1::uuid`, tenantA); err == nil {
			t.Fatal("database allowed AuditEvent deletion")
		}
		directTx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		directPublisherID := parse("0198fc21-ced5-7000-8000-000000000023").String()
		if _, err := directTx.Exec(ctx, `INSERT INTO public.publishers
  (tenant_id, publisher_id, slug, current_state_version, created_at)
	VALUES ($1::uuid, $2::uuid, 'direct-unaudited', 1, $3)`, tenantA, directPublisherID, now); err != nil {
			t.Fatal(err)
		}
		if _, err := directTx.Exec(ctx, `INSERT INTO public.publisher_state_records
  (tenant_id, publisher_id, version, state, recorded_at)
 VALUES ($1::uuid, $2::uuid, 1, 'claimed', $3)`, tenantA, directPublisherID, now); err != nil {
			t.Fatal(err)
		}
		commitErr := directTx.Commit(ctx)
		var postgresError *pgconn.PgError
		if !errors.As(commitErr, &postgresError) || postgresError.ConstraintName != "mutation_audit_required" {
			t.Fatal("database committed an authoritative mutation without audit")
		}
		var unauditedCount int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.publishers WHERE tenant_id = $1::uuid AND publisher_id = $2::uuid`, tenantA, directPublisherID).Scan(&unauditedCount); err != nil || unauditedCount != 0 {
			t.Fatal("failed unaudited transaction retained domain state")
		}
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
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.namespace_delegations TO db003_service")
		execSQL("GRANT SELECT, INSERT ON public.namespace_delegation_state_records TO db003_service")
		execSQL("GRANT SELECT, INSERT ON public.audit_events TO db003_service")
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
		if _, err := publisherRepository.ChangeState(ctx, a, publisherAID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, b, publisherBID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
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

		delegationID := parse("0198fc21-ced5-7000-8000-000000000055")
		invalidDelegation, _ := domainnamespace.NewDelegation(a, delegationID, namespaceAID, namespaceA.Path(), claimedPublisherID, "acme/security/tools", now.Add(3*time.Minute))
		if err := repository.CreateDelegation(ctx, invalidDelegation); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("unverified delegate class = %q: %v", typedClass(err), err)
		}
		if _, err := publisherRepository.ChangeState(ctx, a, claimedPublisherID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(3*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := repository.CreateDelegation(ctx, invalidDelegation); err != nil {
			t.Fatal(err)
		}
		sibling, _ := domainnamespace.NewDelegation(a, parse("0198fc21-ced5-7000-8000-000000000058"), namespaceAID, namespaceA.Path(), claimedPublisherID, "acme/security/finance", now.Add(3*time.Minute))
		if err := repository.CreateDelegation(ctx, sibling); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("ambiguous sibling delegation class = %q: %v", typedClass(err), err)
		}
		nested, _ := domainnamespace.NewDelegation(a, parse("0198fc21-ced5-7000-8000-000000000059"), namespaceAID, namespaceA.Path(), publisherAID, "acme/security/tools/reviews", now.Add(3*time.Minute))
		if err := repository.CreateDelegation(ctx, nested); err != nil {
			t.Fatalf("strictly nested delegation: %v", err)
		}
		if owner, err := repository.ResolveOwner(ctx, a, "acme/security/tools/reviews/check"); err != nil || owner != publisherAID {
			t.Fatalf("nested longest-prefix resolution: %s %v", owner, err)
		}
		if owner, err := repository.ResolveOwner(ctx, a, "acme/security/other"); err != nil || owner != publisherAID {
			t.Fatalf("root owner resolution: %s %v", owner, err)
		}
		if owner, err := repository.ResolveOwner(ctx, a, "acme/security/tools/reviewer"); err != nil || owner != claimedPublisherID {
			t.Fatalf("delegated longest-prefix resolution: %s %v", owner, err)
		}
		if _, err := publisherRepository.ChangeState(ctx, a, claimedPublisherID, 2, domainpublisher.StateSuspended, reason, "paused", now.Add(4*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.ResolveOwner(ctx, a, "acme/security/tools/reviewer"); typedClass(err) != shared.ErrorForbidden {
			t.Fatalf("inactive controlling Publisher did not fail closed: %v", err)
		}
		if _, err := publisherRepository.ChangeState(ctx, a, claimedPublisherID, 3, domainpublisher.StateVerified, reason, "restored", now.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		collision, _ := domainnamespace.New(a, parse("0198fc21-ced5-7000-8000-000000000056"), "acme/security/tools", publisherAID, now.Add(4*time.Minute))
		if err := repository.Create(ctx, collision); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("namespace/delegation collision class = %q: %v", typedClass(err), err)
		}
		revokeReason, _ := shared.NewReasonCode("ownership.changed")
		revoked, err := repository.RevokeDelegation(ctx, a, delegationID, 1, revokeReason, "team changed", now.Add(6*time.Minute))
		if err != nil || revoked.State() != domainnamespace.DelegationRevoked || revoked.StateVersion() != 2 {
			t.Fatalf("delegation revocation: %#v %v", revoked, err)
		}
		if _, err := repository.RevokeDelegation(ctx, a, delegationID, 1, revokeReason, "stale", now.Add(7*time.Minute)); typedCode(err) != "namespace.stale_delegation_version" {
			t.Fatalf("stale delegation version code = %q: %v", typedCode(err), err)
		}
		if owner, err := repository.ResolveOwner(ctx, a, "acme/security/tools/reviewer"); err != nil || owner != publisherAID {
			t.Fatalf("revoked delegation did not fall back to root: %s %v", owner, err)
		}
		if _, err := repository.GetDelegation(ctx, b, delegationID); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant delegation get class = %q: %v", typedClass(err), err)
		}
		reassignment, _ := domainnamespace.NewDelegation(a, parse("0198fc21-ced5-7000-8000-000000000057"), namespaceAID, namespaceA.Path(), claimedPublisherID, "acme/security/tools", now.Add(8*time.Minute))
		if err := repository.CreateDelegation(ctx, reassignment); err != nil {
			t.Fatalf("append-only reassignment: %v", err)
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
		execSQL("GRANT SELECT, INSERT ON public.audit_events TO db004_service")
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
		if _, err := publisherRepository.ChangeState(ctx, tenantA, publisherAID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, tenantB, publisherBID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
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
		values, err := repository.List(ctx, tenantA, "review", nil, 50)
		if err != nil || len(values) != 1 || values[0].TenantID() != tenantA {
			t.Fatalf("tenant list: %#v %v", values, err)
		}
		values, err = repository.List(ctx, tenantA, "%", nil, 50)
		if err != nil || len(values) != 0 {
			t.Fatalf("literal wildcard query: %#v %v", values, err)
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
		execSQL("GRANT SELECT, INSERT ON public.audit_events TO db005_service")
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
		if _, err := publisherRepository.ChangeState(ctx, tenantA, publisherAID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := publisherRepository.ChangeState(ctx, tenantB, publisherBID, 1, domainpublisher.StateVerified, reason, "checked", now.Add(time.Minute)); err != nil {
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
	t.Run("idempotency_repository", func(t *testing.T) {
		conn := newDB(t, "idempotency_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		tenantAValue := "0198fc21-ced5-7000-8000-000000000210"
		tenantBValue := "0198fc21-ced5-7000-8000-000000000211"
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantAValue, tenantBValue))
		execSQL("CREATE ROLE db012_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.idempotency_records TO db012_service")
		execSQL("SET ROLE db012_service")

		parse := func(value string) shared.UUID {
			parsed, parseErr := shared.ParseUUID(value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return parsed
		}
		tenantA, tenantB := parse(tenantAValue), parse(tenantBValue)
		repository, err := postgresidempotency.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		action, _ := shared.NewReasonCode("publisher.create")
		now := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
		digest := shared.SHA256Digest([]byte(`{"display_name":"Acme"}`))
		record, _ := domainidempotency.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000212"),
			"oidc:issuer:subject", action, "publisher-request-1", digest, now, now.Add(24*time.Hour))
		stored, created, err := repository.Acquire(ctx, record)
		if err != nil || !created || stored.ID() != record.ID() || stored.State() != domainidempotency.StatePending {
			t.Fatalf("acquire: %#v %v %v", stored, created, err)
		}
		duplicate, _ := domainidempotency.New(tenantA, parse("0198fc21-ced5-7000-8000-000000000213"),
			"oidc:issuer:subject", action, "publisher-request-1", digest, now, now.Add(24*time.Hour))
		stored, created, err = repository.Acquire(ctx, duplicate)
		if err != nil || created || stored.ID() != record.ID() {
			t.Fatalf("duplicate acquire: %#v %v %v", stored, created, err)
		}
		mismatch, _ := domainidempotency.New(tenantA, duplicate.ID(), duplicate.Principal(), action,
			duplicate.Key(), shared.SHA256Digest([]byte("different")), now, now.Add(24*time.Hour))
		if _, _, err := repository.Acquire(ctx, mismatch); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("request mismatch class = %q: %v", typedClass(err), err)
		}
		if _, err := repository.Get(ctx, tenantB, record.Principal(), action, record.Key()); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}
		result, _ := domainidempotency.NewResult(201, "publisher", "0198fc21-ced5-7000-8000-000000000214")
		completed, err := repository.Complete(ctx, tenantA, record.Principal(), action, record.Key(), digest, result, now.Add(time.Second))
		if established, ok := completed.Result(); err != nil || !ok || established.Status() != 201 || completed.State() != domainidempotency.StateCompleted {
			t.Fatalf("complete: %#v %v", completed, err)
		}
		if _, err := repository.Complete(ctx, tenantA, record.Principal(), action, record.Key(), digest, result, now.Add(2*time.Second)); err != nil {
			t.Fatalf("repeat completion: %v", err)
		}
		differentResult, _ := domainidempotency.NewResult(200, "publisher", "0198fc21-ced5-7000-8000-000000000214")
		if _, err := repository.Complete(ctx, tenantA, record.Principal(), action, record.Key(), digest, differentResult, now.Add(2*time.Second)); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("result mismatch class = %q: %v", typedClass(err), err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.idempotency_records SET request_digest = $1
 WHERE tenant_id = $2::uuid AND idempotency_record_id = $3::uuid`, shared.SHA256Digest([]byte("rewritten")).String(), tenantA.String(), record.ID().String()); err == nil {
			t.Fatal("database allowed idempotency ownership mutation")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.idempotency_records
  (tenant_id, idempotency_record_id, principal_id, action, idempotency_key, request_digest, created_at, expires_at)
 VALUES ($1::uuid, $2::uuid, 'principal', 'publisher.create', 'short-retention', $3, $4, $4 + INTERVAL '23 hours')`,
			tenantA.String(), parse("0198fc21-ced5-7000-8000-000000000215").String(), digest.String(), now); err == nil {
			t.Fatal("database accepted less than 24-hour retention")
		}
	})
	t.Run("outbox_repository", func(t *testing.T) {
		conn := newDB(t, "outbox_test")
		if _, err := Run(ctx, conn, migrations.Files, true); err != nil {
			t.Fatal(err)
		}
		execSQL := func(sql string) {
			t.Helper()
			if _, err := conn.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
		tenantAValue := "0198fc21-ced5-7000-8000-000000000220"
		tenantBValue := "0198fc21-ced5-7000-8000-000000000221"
		execSQL(fmt.Sprintf("INSERT INTO public.tenants (tenant_id) VALUES ('%s'), ('%s')", tenantAValue, tenantBValue))
		execSQL("CREATE ROLE db013_service NOSUPERUSER NOBYPASSRLS NOLOGIN")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.tenant_event_sequences TO db013_service")
		execSQL("GRANT SELECT, INSERT, UPDATE ON public.outbox_messages TO db013_service")
		execSQL("SET ROLE db013_service")

		parse := func(value string) shared.UUID {
			parsed, parseErr := shared.ParseUUID(value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return parsed
		}
		tenantA, tenantB := parse(tenantAValue), parse(tenantBValue)
		repository, err := postgresoutbox.NewRepository(conn)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
		appendMessage := func(tenantID, messageID shared.UUID, at time.Time) domainoutbox.Message {
			t.Helper()
			tx, beginErr := conn.Begin(ctx)
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, setErr := tx.Exec(ctx, `SELECT set_config('thinkpixelmp.tenant_id', $1, true)`, tenantID.String()); setErr != nil {
				t.Fatal(setErr)
			}
			sequence, sequenceErr := postgresoutbox.NextSequence(ctx, tx, tenantID)
			if sequenceErr != nil {
				t.Fatal(sequenceErr)
			}
			payload := []byte(fmt.Sprintf(`{"specversion":"1.0","id":"%s","source":"urn:thinkpixel:mp:integration","type":"io.thinkpixel.mp.artifact.registered.v1","subject":"%s","time":"%s","datacontenttype":"%s","sequence":%d,"data":{"tenant_id":"%s","transaction_cursor":"cursor-%d","artifact_version_id":"%s","artifact_digest":"sha256:%s","descriptor_digest":"sha256:%s"}}`,
				messageID.String(), messageID.String(), at.Format(time.RFC3339Nano), domainoutbox.DataContentType, sequence,
				tenantID.String(), sequence, messageID.String(), strings.Repeat("a", 64), strings.Repeat("b", 64)))
			message, messageErr := domainoutbox.New(tenantID, messageID, sequence, "urn:thinkpixel:mp:integration", "io.thinkpixel.mp.artifact.registered.v1", messageID.String(), payload, at)
			if messageErr != nil {
				t.Fatal(messageErr)
			}
			if recordErr := postgresoutbox.Record(ctx, tx, message); recordErr != nil {
				t.Fatal(recordErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				t.Fatal(commitErr)
			}
			return message
		}
		messageA := appendMessage(tenantA, parse("0198fc21-ced5-7000-8000-000000000222"), now)
		messageB := appendMessage(tenantB, parse("0198fc21-ced5-7000-8000-000000000223"), now)
		if messageA.Sequence() != 1 || messageB.Sequence() != 1 {
			t.Fatal("tenant-local sequence did not start at one")
		}
		if _, err := repository.Get(ctx, tenantA, messageB.ID()); typedClass(err) != shared.ErrorNotFound {
			t.Fatalf("cross-tenant get class = %q: %v", typedClass(err), err)
		}
		claimOne := parse("0198fc21-ced5-7000-8000-000000000224")
		claimed, err := repository.Claim(ctx, tenantA, "worker-a", claimOne, now.Add(time.Second), time.Minute, 10)
		if err != nil || len(claimed) != 1 || claimed[0].Attempts() != 1 || claimed[0].PayloadDigest() != messageA.PayloadDigest() {
			t.Fatalf("claim: %#v %v", claimed, err)
		}
		wrongToken := parse("0198fc21-ced5-7000-8000-000000000225")
		if _, err := repository.Deliver(ctx, tenantA, messageA.ID(), wrongToken, now.Add(2*time.Second)); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("stale completion class = %q: %v", typedClass(err), err)
		}
		retryReason, _ := shared.NewReasonCode("sink.unavailable")
		retryAt := now.Add(2 * time.Minute)
		retried, err := repository.Retry(ctx, tenantA, messageA.ID(), claimOne, retryReason, retryAt)
		if lastError, ok := retried.LastError(); err != nil || retried.State() != domainoutbox.StateRetry || !ok || lastError != retryReason {
			t.Fatalf("retry: %#v %v", retried, err)
		}
		claimed, err = repository.Claim(ctx, tenantA, "worker-a", wrongToken, retryAt.Add(-time.Second), time.Minute, 10)
		if err != nil || len(claimed) != 0 {
			t.Fatalf("early retry claim: %#v %v", claimed, err)
		}
		claimed, err = repository.Claim(ctx, tenantA, "worker-a", wrongToken, retryAt, time.Minute, 10)
		if err != nil || len(claimed) != 1 || claimed[0].Attempts() != 2 {
			t.Fatalf("retry claim: %#v %v", claimed, err)
		}
		claimThree := parse("0198fc21-ced5-7000-8000-000000000226")
		claimed, err = repository.Claim(ctx, tenantA, "worker-b", claimThree, retryAt.Add(time.Minute), time.Minute, 10)
		if err != nil || len(claimed) != 1 || claimed[0].Attempts() != 3 {
			t.Fatalf("lease reclaim: %#v %v", claimed, err)
		}
		if _, err := repository.Deliver(ctx, tenantA, messageA.ID(), wrongToken, retryAt.Add(time.Minute)); typedClass(err) != shared.ErrorConflict {
			t.Fatalf("expired claim completion class = %q: %v", typedClass(err), err)
		}
		deadReason, _ := shared.NewReasonCode("sink.rejected")
		dead, err := repository.DeadLetter(ctx, tenantA, messageA.ID(), claimThree, deadReason, retryAt.Add(time.Minute))
		retainUntil, retained := dead.RetainUntil()
		if err != nil || dead.State() != domainoutbox.StateDeadLetter || !retained || retainUntil.Before(retryAt.Add(time.Minute).Add(domainoutbox.DeadLetterRetention)) {
			t.Fatalf("dead letter: %#v %v", dead, err)
		}

		messageTwo := appendMessage(tenantA, parse("0198fc21-ced5-7000-8000-000000000227"), now.Add(time.Second))
		claimFour := parse("0198fc21-ced5-7000-8000-000000000228")
		claimed, err = repository.Claim(ctx, tenantA, "worker-a", claimFour, now.Add(2*time.Second), time.Minute, 10)
		if err != nil || len(claimed) != 1 || claimed[0].ID() != messageTwo.ID() || messageTwo.Sequence() != 2 {
			t.Fatalf("ordered second claim: %#v %v", claimed, err)
		}
		delivered, err := repository.Deliver(ctx, tenantA, messageTwo.ID(), claimFour, now.Add(3*time.Second))
		if _, retained := delivered.RetainUntil(); err != nil || delivered.State() != domainoutbox.StateDelivered || !retained {
			t.Fatalf("deliver: %#v %v", delivered, err)
		}

		execSQL("RESET ROLE")
		if _, err := conn.Exec(ctx, `UPDATE public.outbox_messages SET event_payload = $1 WHERE tenant_id = $2::uuid AND outbox_message_id = $3::uuid`, []byte(`{}`), tenantA.String(), messageA.ID().String()); err == nil {
			t.Fatal("database allowed event payload mutation")
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.outbox_messages
 (tenant_id, outbox_message_id, sequence, event_source, event_type, event_subject, event_payload, payload_digest, available_at, created_at)
 VALUES ($1::uuid, $2::uuid, 99, 'urn:test', 'io.thinkpixel.mp.artifact.registered.v1', 'subject', $3, $4, $5, $5)`,
			tenantA.String(), parse("0198fc21-ced5-7000-8000-000000000229").String(), messageA.Payload(), messageA.PayloadDigest().String(), now); err == nil {
			t.Fatal("database accepted an unallocated event sequence")
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
			expected := "000001_tenants.sql " + want + "\n000002_publishers.sql " + want + "\n000003_namespaces.sql " + want + "\n000004_artifacts.sql " + want + "\n000005_artifact_versions.sql " + want + "\n000006_artifact_version_mutation_guards.sql " + want + "\n000007_artifact_sources.sql " + want + "\n000008_artifact_descriptors.sql " + want + "\n000009_artifact_requirements.sql " + want + "\n000010_artifact_dependencies.sql " + want + "\n000011_audit_events.sql " + want + "\n000012_idempotency_records.sql " + want + "\n000013_outbox_messages.sql " + want + "\n000014_namespace_delegations.sql " + want
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
		if _, err := conn.Exec(ctx, "INSERT INTO public.schema_migrations (name, checksum) VALUES ('000012_unknown.sql', repeat('0',64))"); err != nil {
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
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&n); err != nil || n != 14 {
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

func typedCode(err error) string {
	var typed *shared.TypedError
	if errors.As(err, &typed) {
		return typed.Code().String()
	}
	return ""
}
