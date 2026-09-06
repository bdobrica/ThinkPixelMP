CREATE TABLE public.namespace_delegations (
    tenant_id uuid NOT NULL,
    delegation_id uuid NOT NULL CHECK (uuid_extract_version(delegation_id) IS NOT DISTINCT FROM 7),
    namespace_id uuid NOT NULL,
    child_prefix text NOT NULL CHECK (
        octet_length(child_prefix) BETWEEN 1 AND 255
        AND child_prefix ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$'
    ),
    publisher_id uuid NOT NULL,
    current_state_version bigint NOT NULL DEFAULT 1 CHECK (current_state_version > 0),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, delegation_id),
    UNIQUE (tenant_id, delegation_id, child_prefix),
    FOREIGN KEY (tenant_id, namespace_id)
        REFERENCES public.namespaces (tenant_id, namespace_id),
    FOREIGN KEY (tenant_id, publisher_id)
        REFERENCES public.publishers (tenant_id, publisher_id),
    CHECK ((current_state_version = 1 AND revoked_at IS NULL) OR
           (current_state_version = 2 AND revoked_at IS NOT NULL AND revoked_at >= created_at))
);

CREATE UNIQUE INDEX namespace_delegations_active_prefix_key
    ON public.namespace_delegations (tenant_id, child_prefix)
    WHERE revoked_at IS NULL;

CREATE TABLE public.namespace_delegation_state_records (
    tenant_id uuid NOT NULL,
    delegation_id uuid NOT NULL,
    child_prefix text NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    state text NOT NULL CHECK (state IN ('active', 'revoked')),
    reason_code text CHECK (reason_code IS NULL OR
        (reason_code ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(reason_code) <= 128)),
    explanation text NOT NULL DEFAULT '' CHECK (octet_length(explanation) <= 4096 AND explanation !~ '[[:cntrl:]]'),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, delegation_id, version),
    FOREIGN KEY (tenant_id, delegation_id, child_prefix)
        REFERENCES public.namespace_delegations (tenant_id, delegation_id, child_prefix),
    CHECK ((version = 1 AND state = 'active' AND reason_code IS NULL AND explanation = '') OR
           (version = 2 AND state = 'revoked' AND reason_code IS NOT NULL))
);

ALTER TABLE public.namespace_delegations
    ADD CONSTRAINT namespace_delegations_current_state_fkey
    FOREIGN KEY (tenant_id, delegation_id, current_state_version)
    REFERENCES public.namespace_delegation_state_records (tenant_id, delegation_id, version)
    DEFERRABLE INITIALLY DEFERRED;

CREATE FUNCTION public.enforce_namespace_delegation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    root_path text;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text || ':' || NEW.namespace_id::text, 1));
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text || ':' || NEW.child_prefix, 0));
    SELECT path INTO root_path
      FROM public.namespaces
     WHERE tenant_id = NEW.tenant_id AND namespace_id = NEW.namespace_id;
    IF root_path IS NULL OR NEW.child_prefix NOT LIKE root_path || '/%' THEN
        RAISE EXCEPTION 'delegation must be a strict child of its namespace'
            USING ERRCODE = '23514', CONSTRAINT = 'namespace_delegations_strict_child_check';
    END IF;
    IF EXISTS (SELECT 1 FROM public.namespaces
                WHERE tenant_id = NEW.tenant_id AND path = NEW.child_prefix) THEN
        RAISE EXCEPTION 'delegation collides with a namespace ownership root'
            USING ERRCODE = '23505', CONSTRAINT = 'namespace_delegations_namespace_collision';
    END IF;
    IF EXISTS (
        SELECT 1 FROM public.namespace_delegations d
         WHERE d.tenant_id = NEW.tenant_id AND d.namespace_id = NEW.namespace_id
           AND d.revoked_at IS NULL
           AND NEW.child_prefix NOT LIKE d.child_prefix || '/%'
           AND d.child_prefix NOT LIKE NEW.child_prefix || '/%'
    ) THEN
        RAISE EXCEPTION 'active delegation prefixes must be strictly nested'
            USING ERRCODE = '23514', CONSTRAINT = 'namespace_delegations_ambiguous_sibling_check';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.publishers p
        JOIN public.publisher_state_records s
          ON s.tenant_id = p.tenant_id AND s.publisher_id = p.publisher_id
         AND s.version = p.current_state_version
        WHERE p.tenant_id = NEW.tenant_id AND p.publisher_id = NEW.publisher_id
          AND s.state = 'verified' FOR SHARE OF p
    ) THEN
        RAISE EXCEPTION 'delegation recipient must be a verified tenant publisher'
            USING ERRCODE = '23514', CONSTRAINT = 'namespace_delegations_verified_publisher_check';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_namespace_delegation() FROM PUBLIC;
CREATE TRIGGER namespace_delegation_insert_guard
BEFORE INSERT ON public.namespace_delegations
FOR EACH ROW EXECUTE FUNCTION public.enforce_namespace_delegation();

CREATE FUNCTION public.reject_namespace_delegation_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.delegation_id IS DISTINCT FROM OLD.delegation_id
       OR NEW.namespace_id IS DISTINCT FROM OLD.namespace_id
       OR NEW.child_prefix IS DISTINCT FROM OLD.child_prefix
       OR NEW.publisher_id IS DISTINCT FROM OLD.publisher_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR OLD.current_state_version <> 1 OR NEW.current_state_version <> 2
       OR OLD.revoked_at IS NOT NULL OR NEW.revoked_at IS NULL THEN
        RAISE EXCEPTION 'namespace delegation identity and history are append-only';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_namespace_delegation_mutation() FROM PUBLIC;
CREATE TRIGGER namespace_delegation_mutation_guard
BEFORE UPDATE OR DELETE ON public.namespace_delegations
FOR EACH ROW EXECUTE FUNCTION public.reject_namespace_delegation_mutation();

CREATE FUNCTION public.reject_namespace_delegation_state_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'namespace delegation state history is append-only';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_namespace_delegation_state_mutation() FROM PUBLIC;
CREATE TRIGGER namespace_delegation_state_mutation_guard
BEFORE UPDATE OR DELETE ON public.namespace_delegation_state_records
FOR EACH ROW EXECUTE FUNCTION public.reject_namespace_delegation_state_mutation();

CREATE FUNCTION public.reject_namespace_delegation_namespace_collision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text || ':' || NEW.path, 0));
    IF EXISTS (SELECT 1 FROM public.namespace_delegations
                WHERE tenant_id = NEW.tenant_id AND child_prefix = NEW.path AND revoked_at IS NULL) THEN
        RAISE EXCEPTION 'namespace ownership root collides with an active delegation'
            USING ERRCODE = '23505', CONSTRAINT = 'namespaces_delegation_collision';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_namespace_delegation_namespace_collision() FROM PUBLIC;
CREATE TRIGGER namespace_delegation_collision_guard
BEFORE INSERT ON public.namespaces
FOR EACH ROW EXECUTE FUNCTION public.reject_namespace_delegation_namespace_collision();

ALTER TABLE public.namespace_delegations ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.namespace_delegations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.namespace_delegations
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);
ALTER TABLE public.namespace_delegation_state_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.namespace_delegation_state_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.namespace_delegation_state_records
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE CONSTRAINT TRIGGER namespace_delegation_create_audit_required
AFTER INSERT ON public.namespace_delegations DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.require_mutation_audit('namespace_delegation', 'namespace.delegated', 'delegation_id');

CREATE FUNCTION public.require_namespace_delegation_revocation_audit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.audit_events event
         WHERE event.tenant_id = NEW.tenant_id
           AND event.transaction_id = pg_current_xact_id()
           AND event.action = 'namespace.delegation_revoked'
           AND event.resource_type = 'namespace_delegation'
           AND event.resource_id = NEW.delegation_id::text
           AND event.decision = 'revoked'
           AND NEW.reason_code = ANY(event.reason_codes)
    ) THEN
        RAISE EXCEPTION 'delegation revocation requires a transactionally coupled audit event'
            USING ERRCODE = '23514', CONSTRAINT = 'mutation_audit_required';
    END IF;
    RETURN NULL;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.require_namespace_delegation_revocation_audit() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER namespace_delegation_revoke_audit_required
AFTER INSERT ON public.namespace_delegation_state_records DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW WHEN (NEW.version > 1)
EXECUTE FUNCTION public.require_namespace_delegation_revocation_audit();
