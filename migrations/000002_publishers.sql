CREATE TABLE public.publishers (
    tenant_id uuid NOT NULL,
    publisher_id uuid NOT NULL CHECK (uuid_extract_version(publisher_id) IS NOT DISTINCT FROM 7),
    slug text NOT NULL CHECK (
        octet_length(slug) BETWEEN 1 AND 63
        AND slug ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$'
    ),
    display_name text CHECK (
        octet_length(display_name) BETWEEN 1 AND 256
        AND display_name !~ '[[:cntrl:]]'
    ),
    description text CHECK (
        octet_length(description) <= 4096
        AND description !~ '[[:cntrl:]]'
    ),
    current_state_version bigint NOT NULL DEFAULT 1 CHECK (current_state_version > 0),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, publisher_id),
    UNIQUE (tenant_id, slug),
    FOREIGN KEY (tenant_id) REFERENCES public.tenants (tenant_id)
);

CREATE TABLE public.publisher_state_records (
    tenant_id uuid NOT NULL,
    publisher_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    state text NOT NULL CHECK (state IN ('claimed', 'verified', 'suspended', 'revoked')),
    reason_code text CHECK (
        octet_length(reason_code) BETWEEN 1 AND 128
        AND reason_code ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$'
    ),
    explanation text CHECK (
        octet_length(explanation) <= 4096
        AND explanation !~ '[[:cntrl:]]'
    ),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, publisher_id, version),
    FOREIGN KEY (tenant_id, publisher_id)
        REFERENCES public.publishers (tenant_id, publisher_id)
);

ALTER TABLE public.publishers
    ADD CONSTRAINT publishers_current_state_fk
    FOREIGN KEY (tenant_id, publisher_id, current_state_version)
    REFERENCES public.publisher_state_records (tenant_id, publisher_id, version)
    DEFERRABLE INITIALLY DEFERRED;

CREATE FUNCTION public.enforce_publisher_state_record() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    previous_state text;
BEGIN
    IF NEW.version = 1 THEN
        IF NEW.state <> 'claimed' OR NEW.reason_code IS NOT NULL OR NEW.explanation IS NOT NULL THEN
            RAISE EXCEPTION 'invalid initial publisher state';
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.reason_code IS NULL THEN
        RAISE EXCEPTION 'publisher state transition reason is required';
    END IF;

    SELECT state INTO previous_state
      FROM public.publisher_state_records
     WHERE tenant_id = NEW.tenant_id
       AND publisher_id = NEW.publisher_id
       AND version = NEW.version - 1;

    IF previous_state IS NULL OR NOT (
        (previous_state = 'claimed' AND NEW.state IN ('verified', 'suspended', 'revoked'))
        OR (previous_state = 'verified' AND NEW.state IN ('suspended', 'revoked'))
        OR (previous_state = 'suspended' AND NEW.state IN ('verified', 'revoked'))
    ) THEN
        RAISE EXCEPTION 'invalid publisher state transition';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_publisher_state_record() FROM PUBLIC;

CREATE TRIGGER publisher_state_record_insert_guard
BEFORE INSERT ON public.publisher_state_records
FOR EACH ROW EXECUTE FUNCTION public.enforce_publisher_state_record();

CREATE FUNCTION public.reject_publisher_state_record_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'publisher state records are append-only';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_publisher_state_record_mutation() FROM PUBLIC;

CREATE TRIGGER publisher_state_record_update_guard
BEFORE UPDATE OR DELETE ON public.publisher_state_records
FOR EACH ROW EXECUTE FUNCTION public.reject_publisher_state_record_mutation();

CREATE FUNCTION public.enforce_publisher_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.publisher_id IS DISTINCT FROM OLD.publisher_id
       OR NEW.slug IS DISTINCT FROM OLD.slug
       OR NEW.display_name IS DISTINCT FROM OLD.display_name
       OR NEW.description IS DISTINCT FROM OLD.description
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.current_state_version <> OLD.current_state_version + 1 THEN
        RAISE EXCEPTION 'publisher identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_publisher_update() FROM PUBLIC;

CREATE TRIGGER publisher_update_guard
BEFORE UPDATE ON public.publishers
FOR EACH ROW EXECUTE FUNCTION public.enforce_publisher_update();

ALTER TABLE public.publishers ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.publishers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.publishers
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

ALTER TABLE public.publisher_state_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.publisher_state_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.publisher_state_records
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);
