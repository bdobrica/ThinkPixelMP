CREATE TABLE public.tenant_event_sequences (
    tenant_id uuid PRIMARY KEY,
    next_sequence bigint NOT NULL DEFAULT 1 CHECK (next_sequence BETWEEN 1 AND 9223372036854775807),
    FOREIGN KEY (tenant_id) REFERENCES public.tenants (tenant_id)
);

ALTER TABLE public.tenant_event_sequences ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.tenant_event_sequences FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.tenant_event_sequences
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE TABLE public.outbox_messages (
    tenant_id uuid NOT NULL,
    outbox_message_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_source text NOT NULL CHECK (char_length(event_source) BETWEEN 1 AND 1024 AND event_source !~ '[[:cntrl:]]'),
    event_type text NOT NULL CHECK (event_type ~ '^io[.]thinkpixel[.]mp[.][a-z0-9](?:[a-z0-9.-]*[a-z0-9])?[.]v1$' AND char_length(event_type) <= 128),
    event_subject text NOT NULL CHECK (char_length(event_subject) BETWEEN 1 AND 512 AND event_subject !~ '[[:cntrl:]]'),
    event_payload bytea NOT NULL CHECK (octet_length(event_payload) BETWEEN 2 AND 262144),
    payload_digest text NOT NULL CHECK (payload_digest ~ '^sha256:[0-9a-f]{64}$' AND payload_digest = 'sha256:' || encode(sha256(event_payload), 'hex')),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'claimed', 'retry', 'delivered', 'dead_letter')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 1000),
    available_at timestamptz NOT NULL,
    claim_token uuid,
    claimed_by text CHECK (claimed_by IS NULL OR (char_length(claimed_by) BETWEEN 1 AND 255 AND claimed_by !~ '[[:cntrl:]]')),
    claimed_at timestamptz,
    lease_expires_at timestamptz,
    last_error_code text CHECK (last_error_code IS NULL OR (last_error_code ~ '^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$' AND char_length(last_error_code) <= 128)),
    delivered_at timestamptz,
    dead_lettered_at timestamptz,
    retain_until timestamptz,
    created_at timestamptz NOT NULL,
    transaction_id xid8 NOT NULL DEFAULT pg_current_xact_id(),
    PRIMARY KEY (tenant_id, outbox_message_id),
    UNIQUE (tenant_id, sequence),
    FOREIGN KEY (tenant_id) REFERENCES public.tenants (tenant_id),
    CHECK (uuid_extract_version(outbox_message_id) IS NOT DISTINCT FROM 7),
    CHECK (claim_token IS NULL OR uuid_extract_version(claim_token) IS NOT DISTINCT FROM 7),
    CHECK (available_at >= created_at),
    CHECK (
        (state = 'pending' AND attempt_count = 0 AND last_error_code IS NULL AND claim_token IS NULL AND claimed_by IS NULL AND claimed_at IS NULL AND lease_expires_at IS NULL AND delivered_at IS NULL AND dead_lettered_at IS NULL AND retain_until IS NULL)
        OR
        (state = 'retry' AND attempt_count > 0 AND last_error_code IS NOT NULL AND claim_token IS NULL AND claimed_by IS NULL AND claimed_at IS NULL AND lease_expires_at IS NULL AND delivered_at IS NULL AND dead_lettered_at IS NULL AND retain_until IS NULL)
        OR
        (state = 'claimed' AND claim_token IS NOT NULL AND claimed_by IS NOT NULL AND claimed_at IS NOT NULL
            AND lease_expires_at > claimed_at AND lease_expires_at <= claimed_at + INTERVAL '15 minutes'
            AND attempt_count > 0 AND delivered_at IS NULL AND dead_lettered_at IS NULL AND retain_until IS NULL)
        OR
        (state = 'delivered' AND claim_token IS NULL AND claimed_by IS NULL AND claimed_at IS NULL AND lease_expires_at IS NULL
            AND delivered_at IS NOT NULL AND dead_lettered_at IS NULL AND retain_until >= delivered_at + INTERVAL '30 days')
        OR
        (state = 'dead_letter' AND claim_token IS NULL AND claimed_by IS NULL AND claimed_at IS NULL AND lease_expires_at IS NULL
            AND delivered_at IS NULL AND dead_lettered_at IS NOT NULL AND last_error_code IS NOT NULL
            AND retain_until >= dead_lettered_at + INTERVAL '90 days')
    )
);

CREATE INDEX outbox_messages_claim_idx
    ON public.outbox_messages (tenant_id, available_at, sequence)
    WHERE state IN ('pending', 'retry', 'claimed');
CREATE INDEX outbox_messages_retention_idx
    ON public.outbox_messages (tenant_id, retain_until)
    WHERE retain_until IS NOT NULL;

ALTER TABLE public.outbox_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.outbox_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.outbox_messages
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.require_allocated_event_sequence() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.tenant_event_sequences
         WHERE tenant_id = NEW.tenant_id AND next_sequence = NEW.sequence + 1
    ) THEN
        RAISE EXCEPTION 'outbox event sequence was not allocated in this transaction';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.require_allocated_event_sequence() FROM PUBLIC;

CREATE TRIGGER outbox_allocated_sequence_required
BEFORE INSERT ON public.outbox_messages
FOR EACH ROW EXECUTE FUNCTION public.require_allocated_event_sequence();

CREATE FUNCTION public.guard_outbox_message_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
       OR OLD.outbox_message_id IS DISTINCT FROM NEW.outbox_message_id
       OR OLD.sequence IS DISTINCT FROM NEW.sequence
       OR OLD.event_source IS DISTINCT FROM NEW.event_source
       OR OLD.event_type IS DISTINCT FROM NEW.event_type
       OR OLD.event_subject IS DISTINCT FROM NEW.event_subject
       OR OLD.event_payload IS DISTINCT FROM NEW.event_payload
       OR OLD.payload_digest IS DISTINCT FROM NEW.payload_digest
       OR OLD.created_at IS DISTINCT FROM NEW.created_at
       OR OLD.transaction_id IS DISTINCT FROM NEW.transaction_id
       OR NEW.attempt_count < OLD.attempt_count
       OR NEW.attempt_count > OLD.attempt_count + 1
       OR (NEW.state = 'claimed' AND NEW.attempt_count <> OLD.attempt_count + 1)
       OR (NEW.state <> 'claimed' AND NEW.attempt_count <> OLD.attempt_count)
       OR (OLD.state = 'claimed' AND NEW.state = 'claimed' AND OLD.lease_expires_at > NEW.claimed_at)
       OR (NEW.state = 'retry' AND (NEW.last_error_code IS NULL OR NEW.available_at <= OLD.claimed_at OR NEW.available_at > OLD.claimed_at + INTERVAL '24 hours'))
       OR (NEW.state = 'delivered' AND NEW.delivered_at < OLD.claimed_at)
       OR (NEW.state = 'dead_letter' AND NEW.dead_lettered_at < OLD.claimed_at)
       OR NOT (
           (OLD.state IN ('pending', 'retry') AND NEW.state = 'claimed')
           OR (OLD.state = 'claimed' AND NEW.state IN ('claimed', 'retry', 'delivered', 'dead_letter'))
       ) THEN
        RAISE EXCEPTION 'outbox update violates immutable event or delivery transition';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.guard_outbox_message_update() FROM PUBLIC;

CREATE TRIGGER outbox_message_update_guard
BEFORE UPDATE ON public.outbox_messages
FOR EACH ROW EXECUTE FUNCTION public.guard_outbox_message_update();
