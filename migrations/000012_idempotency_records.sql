CREATE TABLE public.idempotency_records (
    tenant_id uuid NOT NULL,
    idempotency_record_id uuid NOT NULL,
    principal_id text NOT NULL CHECK (char_length(principal_id) BETWEEN 1 AND 255 AND principal_id !~ '[[:cntrl:]]'),
    action text NOT NULL CHECK (action ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(action) <= 128),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 255 AND idempotency_key !~ '[[:cntrl:]]'),
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'completed')),
    response_status smallint CHECK (response_status BETWEEN 100 AND 599),
    resource_type text CHECK (resource_type IS NULL OR (resource_type ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(resource_type) <= 128)),
    resource_id text CHECK (resource_id IS NULL OR (char_length(resource_id) BETWEEN 1 AND 512 AND resource_id !~ '[[:cntrl:]]')),
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, idempotency_record_id),
    UNIQUE (tenant_id, principal_id, action, idempotency_key),
    FOREIGN KEY (tenant_id) REFERENCES public.tenants (tenant_id),
    CHECK (uuid_extract_version(idempotency_record_id) IS NOT DISTINCT FROM 7),
    CHECK (expires_at >= created_at + INTERVAL '24 hours'),
    CHECK ((resource_type IS NULL) = (resource_id IS NULL)),
    CHECK (
        (state = 'pending' AND response_status IS NULL AND resource_type IS NULL AND completed_at IS NULL)
        OR
        (state = 'completed' AND response_status IS NOT NULL AND completed_at IS NOT NULL AND completed_at >= created_at)
    )
);

CREATE INDEX idempotency_records_expiry_idx
    ON public.idempotency_records (tenant_id, expires_at);

ALTER TABLE public.idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.idempotency_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.idempotency_records
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.guard_idempotency_record_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
       OR OLD.idempotency_record_id IS DISTINCT FROM NEW.idempotency_record_id
       OR OLD.principal_id IS DISTINCT FROM NEW.principal_id
       OR OLD.action IS DISTINCT FROM NEW.action
       OR OLD.idempotency_key IS DISTINCT FROM NEW.idempotency_key
       OR OLD.request_digest IS DISTINCT FROM NEW.request_digest
       OR OLD.created_at IS DISTINCT FROM NEW.created_at
       OR OLD.expires_at IS DISTINCT FROM NEW.expires_at
       OR OLD.state <> 'pending'
       OR NEW.state <> 'completed'
       OR NEW.response_status IS NULL
       OR NEW.completed_at IS NULL THEN
        RAISE EXCEPTION 'idempotency records permit only one pending-to-completed transition';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.guard_idempotency_record_update() FROM PUBLIC;

CREATE TRIGGER idempotency_record_update_guard
BEFORE UPDATE ON public.idempotency_records
FOR EACH ROW EXECUTE FUNCTION public.guard_idempotency_record_update();
