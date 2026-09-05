CREATE TABLE public.artifacts (
    tenant_id uuid NOT NULL,
    artifact_id uuid NOT NULL CHECK (uuid_extract_version(artifact_id) IS NOT DISTINCT FROM 7),
    namespace_id uuid NOT NULL,
    name text NOT NULL CHECK (
        octet_length(name) BETWEEN 1 AND 63
        AND name ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$'
    ),
    kind text NOT NULL CHECK (kind IN ('skill', 'agent-runtime', 'mcp-server', 'remote-agent', 'bundle')),
    display_name text CHECK (
        octet_length(display_name) BETWEEN 1 AND 256
        AND display_name !~ '[[:cntrl:]]'
    ),
    description text CHECK (
        octet_length(description) <= 4096
        AND description !~ '[[:cntrl:]]'
    ),
    homepage text CHECK (
        octet_length(homepage) BETWEEN 1 AND 2048
        AND homepage !~ '[[:cntrl:][:space:]]'
    ),
    repository text CHECK (
        octet_length(repository) BETWEEN 1 AND 2048
        AND repository !~ '[[:cntrl:][:space:]]'
    ),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, artifact_id),
    UNIQUE (tenant_id, namespace_id, name),
    FOREIGN KEY (tenant_id, namespace_id)
        REFERENCES public.namespaces (tenant_id, namespace_id)
);

CREATE FUNCTION public.enforce_artifact_labels() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF jsonb_typeof(NEW.labels) <> 'object'
       OR (SELECT count(*) FROM jsonb_each(NEW.labels)) > 64
       OR EXISTS (
           SELECT 1
             FROM jsonb_each(NEW.labels) AS entry(key, value)
            WHERE octet_length(entry.key) NOT BETWEEN 1 AND 128
               OR entry.key !~ '^[a-z0-9](?:[a-z0-9._/-]*[a-z0-9])?$'
               OR jsonb_typeof(entry.value) <> 'string'
               OR octet_length(entry.value #>> '{}') > 512
       ) THEN
        RAISE EXCEPTION 'invalid artifact labels'
            USING ERRCODE = '23514', CONSTRAINT = 'artifacts_labels_check';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_artifact_labels() FROM PUBLIC;

CREATE TRIGGER artifact_labels_guard
BEFORE INSERT OR UPDATE OF labels ON public.artifacts
FOR EACH ROW EXECUTE FUNCTION public.enforce_artifact_labels();

CREATE FUNCTION public.enforce_artifact_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'artifact identity cannot be deleted';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.artifact_id IS DISTINCT FROM OLD.artifact_id
       OR NEW.namespace_id IS DISTINCT FROM OLD.namespace_id
       OR NEW.name IS DISTINCT FROM OLD.name
       OR NEW.kind IS DISTINCT FROM OLD.kind
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'artifact identity and kind are immutable';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_artifact_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifacts
FOR EACH ROW EXECUTE FUNCTION public.enforce_artifact_mutation();

ALTER TABLE public.artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifacts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifacts
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);
