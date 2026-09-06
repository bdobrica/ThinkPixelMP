CREATE TABLE public.audit_events (
    tenant_id uuid NOT NULL,
    audit_event_id uuid NOT NULL DEFAULT uuidv7(),
    actor_id text NOT NULL CHECK (char_length(actor_id) BETWEEN 1 AND 255 AND actor_id !~ '[[:cntrl:]]'),
    action text NOT NULL CHECK (action ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(action) <= 128),
    resource_type text NOT NULL CHECK (resource_type ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(resource_type) <= 128),
    resource_id text NOT NULL CHECK (char_length(resource_id) BETWEEN 1 AND 512 AND resource_id !~ '[[:cntrl:]]'),
    artifact_digest text CHECK (artifact_digest IS NULL OR artifact_digest ~ '^sha256:[0-9a-f]{64}$'),
    decision text CHECK (decision IS NULL OR (decision ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(decision) <= 128)),
    reason_codes text[] NOT NULL DEFAULT '{}'::text[] CHECK (cardinality(reason_codes) <= 32),
    evidence_ids uuid[] NOT NULL DEFAULT '{}'::uuid[] CHECK (cardinality(evidence_ids) <= 32),
    policy_digests text[] NOT NULL DEFAULT '{}'::text[] CHECK (cardinality(policy_digests) <= 32),
    occurred_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    request_id uuid CHECK (request_id IS NULL OR uuid_extract_version(request_id) IS NOT DISTINCT FROM 7),
    trace_id text CHECK (trace_id IS NULL OR trace_id ~ '^[0-9a-f]{32}$'),
    transaction_id xid8 NOT NULL DEFAULT pg_current_xact_id(),
    PRIMARY KEY (tenant_id, audit_event_id),
    FOREIGN KEY (tenant_id) REFERENCES public.tenants (tenant_id),
    CHECK (uuid_extract_version(audit_event_id) IS NOT DISTINCT FROM 7)
);

CREATE FUNCTION public.valid_audit_reason_codes(input_values text[]) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(bool_and(value IS NOT NULL AND value ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(value) <= 128), true)
      FROM unnest(input_values) AS value
$$;

CREATE FUNCTION public.valid_audit_evidence_ids(input_values uuid[]) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(bool_and(value IS NOT NULL AND uuid_extract_version(value) IS NOT DISTINCT FROM 7), true)
      FROM unnest(input_values) AS value
$$;

CREATE FUNCTION public.valid_audit_policy_digests(input_values text[]) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(bool_and(value IS NOT NULL AND value ~ '^sha256:[0-9a-f]{64}$'), true)
      FROM unnest(input_values) AS value
$$;

ALTER TABLE public.audit_events
    ADD CONSTRAINT audit_events_reason_code_values_check CHECK (public.valid_audit_reason_codes(reason_codes)),
    ADD CONSTRAINT audit_events_evidence_id_values_check CHECK (public.valid_audit_evidence_ids(evidence_ids)),
    ADD CONSTRAINT audit_events_policy_digest_values_check CHECK (public.valid_audit_policy_digests(policy_digests));

ALTER TABLE public.audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.audit_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.audit_events
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.reject_audit_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit events are immutable after recording';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_audit_event_mutation() FROM PUBLIC;

CREATE TRIGGER audit_event_mutation_guard
BEFORE UPDATE OR DELETE ON public.audit_events
FOR EACH ROW EXECUTE FUNCTION public.reject_audit_event_mutation();

CREATE FUNCTION public.require_mutation_audit() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    resource_id_value text;
    digest_value text;
BEGIN
    resource_id_value := to_jsonb(NEW) ->> TG_ARGV[2];
    IF TG_NARGS = 4 THEN
        digest_value := to_jsonb(NEW) ->> TG_ARGV[3];
    END IF;
    IF TG_ARGV[1] = 'artifact_dependency.created' THEN
        resource_id_value := resource_id_value || ':' || NEW.dependency_index::text;
    END IF;
    IF TG_ARGV[0] IN ('artifact_descriptor', 'artifact_requirement', 'artifact_dependency') THEN
        SELECT resolved_digest
          INTO digest_value
          FROM public.artifact_versions
         WHERE tenant_id = NEW.tenant_id
           AND artifact_version_id = NEW.artifact_version_id;
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM public.audit_events event
         WHERE event.tenant_id = NEW.tenant_id
           AND event.transaction_id = pg_current_xact_id()
           AND event.action = TG_ARGV[1]
           AND event.resource_type = TG_ARGV[0]
           AND event.resource_id = resource_id_value
           AND (digest_value IS NULL OR event.artifact_digest = digest_value)
           AND (TG_ARGV[1] <> 'publisher.state_changed' OR (
                event.decision = to_jsonb(NEW) ->> 'state'
                AND (to_jsonb(NEW) ->> 'reason_code' IS NULL OR to_jsonb(NEW) ->> 'reason_code' = ANY(event.reason_codes))
           ))
    ) THEN
        RAISE EXCEPTION 'authoritative mutation requires a transactionally coupled audit event'
            USING ERRCODE = '23514', CONSTRAINT = 'mutation_audit_required';
    END IF;
    RETURN NULL;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.require_mutation_audit() FROM PUBLIC;

CREATE CONSTRAINT TRIGGER publisher_create_audit_required
AFTER INSERT ON public.publishers DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('publisher', 'publisher.created', 'publisher_id');

CREATE CONSTRAINT TRIGGER publisher_state_audit_required
AFTER INSERT ON public.publisher_state_records DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW WHEN (NEW.version > 1)
EXECUTE FUNCTION public.require_mutation_audit('publisher', 'publisher.state_changed', 'publisher_id');

CREATE CONSTRAINT TRIGGER namespace_create_audit_required
AFTER INSERT ON public.namespaces DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('namespace', 'namespace.created', 'namespace_id');

CREATE CONSTRAINT TRIGGER artifact_create_audit_required
AFTER INSERT ON public.artifacts DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact', 'artifact.created', 'artifact_id');

CREATE CONSTRAINT TRIGGER artifact_version_create_audit_required
AFTER INSERT ON public.artifact_versions DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact_version', 'artifact_version.registered', 'artifact_version_id', 'resolved_digest');

CREATE CONSTRAINT TRIGGER artifact_source_create_audit_required
AFTER INSERT ON public.artifact_sources DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact_source', 'artifact_source.created', 'artifact_version_id', 'resolved_digest');

CREATE CONSTRAINT TRIGGER artifact_descriptor_create_audit_required
AFTER INSERT ON public.artifact_descriptors DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact_descriptor', 'artifact_descriptor.created', 'artifact_version_id');

CREATE CONSTRAINT TRIGGER artifact_requirement_create_audit_required
AFTER INSERT ON public.artifact_requirements DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact_requirement', 'artifact_requirement.created', 'artifact_version_id');

CREATE CONSTRAINT TRIGGER artifact_dependency_create_audit_required
AFTER INSERT ON public.artifact_dependencies DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('artifact_dependency', 'artifact_dependency.created', 'artifact_version_id');
