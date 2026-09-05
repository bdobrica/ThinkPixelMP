ALTER TABLE public.namespaces
    ADD CONSTRAINT namespaces_descriptor_coordinates_key
    UNIQUE (tenant_id, namespace_id, path);

ALTER TABLE public.artifacts
    ADD CONSTRAINT artifacts_descriptor_coordinates_key
    UNIQUE (tenant_id, artifact_id, namespace_id, name, kind);

ALTER TABLE public.artifact_versions
    ADD CONSTRAINT artifact_versions_descriptor_coordinates_key
    UNIQUE (tenant_id, artifact_version_id, artifact_id, semantic_version, kind);

CREATE TABLE public.artifact_descriptors (
    tenant_id uuid NOT NULL,
    artifact_version_id uuid NOT NULL,
    artifact_id uuid NOT NULL,
    namespace_id uuid NOT NULL,
    schema_version smallint NOT NULL CHECK (schema_version = 1),
    kind text NOT NULL CHECK (kind IN ('skill', 'agent-runtime', 'mcp-server', 'remote-agent', 'bundle')),
    namespace_path text NOT NULL,
    artifact_name text NOT NULL,
    semantic_version text NOT NULL,
    media_type text NOT NULL CHECK (
        (kind = 'skill' AND media_type = 'application/vnd.thinkpixel.skill.manifest.v1+json')
        OR (kind = 'agent-runtime' AND media_type = 'application/vnd.thinkpixel.agent-runtime.manifest.v1+json')
        OR (kind = 'mcp-server' AND media_type = 'application/vnd.thinkpixel.mcp-server.manifest.v1+json')
        OR (kind = 'remote-agent' AND media_type = 'application/vnd.thinkpixel.remote-agent.manifest.v1+json')
        OR (kind = 'bundle' AND media_type = 'application/vnd.thinkpixel.bundle.manifest.v1+json')
    ),
    descriptor_digest text NOT NULL CHECK (descriptor_digest ~ '^sha256:[0-9a-f]{64}$'),
    normalized_bytes bytea NOT NULL CHECK (octet_length(normalized_bytes) BETWEEN 2 AND 1048576),
    normalized_metadata jsonb NOT NULL CHECK (
        jsonb_typeof(normalized_metadata) = 'object'
        AND normalized_metadata ->> 'schema_version' = schema_version::text
        AND normalized_metadata ->> 'kind' = kind
        AND jsonb_typeof(normalized_metadata -> 'artifact') = 'object'
        AND normalized_metadata -> 'artifact' ->> 'namespace' = namespace_path
        AND normalized_metadata -> 'artifact' ->> 'name' = artifact_name
        AND normalized_metadata -> 'artifact' ->> 'version' = semantic_version
        AND jsonb_typeof(normalized_metadata -> 'requirements') = 'object'
        AND jsonb_typeof(normalized_metadata -> 'dependencies') = 'array'
        AND jsonb_array_length(normalized_metadata -> 'dependencies') <= 256
        AND jsonb_typeof(normalized_metadata -> 'spec') = 'object'
        AND normalized_metadata - ARRAY['schema_version', 'kind', 'artifact', 'requirements', 'dependencies', 'spec'] = '{}'::jsonb
        AND (normalized_metadata -> 'artifact') - ARRAY['namespace', 'name', 'version'] = '{}'::jsonb
    ),
    PRIMARY KEY (tenant_id, artifact_version_id),
    UNIQUE (tenant_id, descriptor_digest),
    FOREIGN KEY (tenant_id, namespace_id, namespace_path)
        REFERENCES public.namespaces (tenant_id, namespace_id, path),
    FOREIGN KEY (tenant_id, artifact_id, namespace_id, artifact_name, kind)
        REFERENCES public.artifacts (tenant_id, artifact_id, namespace_id, name, kind),
    FOREIGN KEY (tenant_id, artifact_version_id, artifact_id, semantic_version, kind)
        REFERENCES public.artifact_versions (tenant_id, artifact_version_id, artifact_id, semantic_version, kind)
);

CREATE FUNCTION public.enforce_artifact_descriptor_metadata() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF convert_from(NEW.normalized_bytes, 'UTF8')::jsonb IS DISTINCT FROM NEW.normalized_metadata THEN
        RAISE EXCEPTION 'descriptor bytes and normalized metadata differ'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_descriptors_metadata_check';
    END IF;
    RETURN NEW;
EXCEPTION
    WHEN character_not_in_repertoire OR invalid_text_representation THEN
        RAISE EXCEPTION 'descriptor bytes are not valid UTF-8 JSON'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_descriptors_metadata_check';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_artifact_descriptor_metadata() FROM PUBLIC;

CREATE TRIGGER artifact_descriptor_metadata_guard
BEFORE INSERT ON public.artifact_descriptors
FOR EACH ROW EXECUTE FUNCTION public.enforce_artifact_descriptor_metadata();

ALTER TABLE public.artifact_descriptors ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifact_descriptors FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifact_descriptors
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.reject_artifact_descriptor_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact descriptors are immutable after registration';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_artifact_descriptor_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_descriptor_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifact_descriptors
FOR EACH ROW EXECUTE FUNCTION public.reject_artifact_descriptor_mutation();
