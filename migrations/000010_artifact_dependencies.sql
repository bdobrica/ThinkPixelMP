CREATE TABLE public.artifact_dependencies (
    tenant_id uuid NOT NULL,
    artifact_version_id uuid NOT NULL,
    dependency_index smallint NOT NULL CHECK (dependency_index BETWEEN 0 AND 255),
    schema_version smallint NOT NULL CHECK (schema_version = 1),
    dependency_name text NOT NULL CHECK (
        dependency_name ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$'
        AND octet_length(dependency_name) <= 63
    ),
    artifact_identity text NOT NULL CHECK (
        artifact_identity ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$'
        AND octet_length(artifact_identity) <= 319
    ),
    required boolean NOT NULL,
    catalog text CHECK (
        catalog IS NULL OR (
            catalog ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$'
            AND octet_length(catalog) <= 63
        )
    ),
    source text CHECK (source IS NULL OR char_length(source) BETWEEN 1 AND 255),
    selector_kind text NOT NULL CHECK (selector_kind IN ('digest', 'version', 'range')),
    selector_value text NOT NULL CHECK (
        char_length(selector_value) BETWEEN 1 AND 255
        AND (selector_kind <> 'digest' OR selector_value ~ '^sha256:[0-9a-f]{64}$')
        AND (selector_kind <> 'version' OR selector_value ~ '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$')
        AND (selector_kind <> 'range' OR selector_value NOT IN ('latest', '*'))
    ),
    dependency_digest text NOT NULL CHECK (dependency_digest ~ '^sha256:[0-9a-f]{64}$'),
    normalized_bytes bytea NOT NULL CHECK (octet_length(normalized_bytes) BETWEEN 2 AND 1048576),
    normalized_dependency jsonb NOT NULL CHECK (
        jsonb_typeof(normalized_dependency) = 'object'
        AND normalized_dependency ?& ARRAY['schema_version', 'name', 'artifact', 'required', 'selector']
        AND normalized_dependency ->> 'schema_version' = schema_version::text
        AND normalized_dependency ->> 'name' = dependency_name
        AND normalized_dependency ->> 'artifact' = artifact_identity
        AND normalized_dependency -> 'required' = to_jsonb(required)
        AND normalized_dependency -> 'selector' = jsonb_build_object(selector_kind, selector_value)
        AND normalized_dependency - ARRAY['schema_version', 'name', 'artifact', 'required', 'catalog', 'source', 'selector'] = '{}'::jsonb
        AND (catalog IS NULL) = (NOT normalized_dependency ? 'catalog')
        AND (catalog IS NULL OR normalized_dependency ->> 'catalog' = catalog)
        AND (source IS NULL) = (NOT normalized_dependency ? 'source')
        AND (source IS NULL OR normalized_dependency ->> 'source' = source)
    ),
    PRIMARY KEY (tenant_id, artifact_version_id, dependency_index),
    UNIQUE (tenant_id, artifact_version_id, dependency_name),
    FOREIGN KEY (tenant_id, artifact_version_id)
        REFERENCES public.artifact_descriptors (tenant_id, artifact_version_id)
);

CREATE FUNCTION public.enforce_artifact_dependency_metadata() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    descriptor_dependency jsonb;
BEGIN
    IF convert_from(NEW.normalized_bytes, 'UTF8')::jsonb IS DISTINCT FROM NEW.normalized_dependency THEN
        RAISE EXCEPTION 'dependency bytes and normalized dependency differ'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_dependencies_metadata_check';
    END IF;

    SELECT normalized_metadata -> 'dependencies' -> NEW.dependency_index::integer
      INTO descriptor_dependency
      FROM public.artifact_descriptors
     WHERE tenant_id = NEW.tenant_id
       AND artifact_version_id = NEW.artifact_version_id;

    IF NOT FOUND OR descriptor_dependency IS DISTINCT FROM NEW.normalized_dependency THEN
        RAISE EXCEPTION 'dependency does not match parent descriptor index'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_dependencies_descriptor_check';
    END IF;
    RETURN NEW;
EXCEPTION
    WHEN character_not_in_repertoire OR invalid_text_representation THEN
        RAISE EXCEPTION 'dependency bytes are not valid UTF-8 JSON'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_dependencies_metadata_check';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_artifact_dependency_metadata() FROM PUBLIC;

CREATE TRIGGER artifact_dependency_metadata_guard
BEFORE INSERT ON public.artifact_dependencies
FOR EACH ROW EXECUTE FUNCTION public.enforce_artifact_dependency_metadata();

ALTER TABLE public.artifact_dependencies ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifact_dependencies FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifact_dependencies
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.reject_artifact_dependency_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact dependencies are immutable after registration';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_artifact_dependency_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_dependency_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifact_dependencies
FOR EACH ROW EXECUTE FUNCTION public.reject_artifact_dependency_mutation();
