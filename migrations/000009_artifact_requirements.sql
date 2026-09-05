CREATE TABLE public.artifact_requirements (
    tenant_id uuid NOT NULL,
    artifact_version_id uuid NOT NULL,
    schema_version smallint NOT NULL CHECK (schema_version = 1),
    requirement_digest text NOT NULL CHECK (requirement_digest ~ '^sha256:[0-9a-f]{64}$'),
    normalized_bytes bytea NOT NULL CHECK (octet_length(normalized_bytes) BETWEEN 2 AND 1048576),
    normalized_requirement jsonb NOT NULL CHECK (
        jsonb_typeof(normalized_requirement) = 'object'
        AND normalized_requirement ? 'schema_version'
        AND normalized_requirement ->> 'schema_version' = schema_version::text
        AND normalized_requirement - ARRAY['schema_version', 'capabilities', 'runtime', 'network', 'integrations'] = '{}'::jsonb
        AND (NOT (normalized_requirement ? 'capabilities') OR jsonb_typeof(normalized_requirement -> 'capabilities') = 'object')
        AND (NOT (normalized_requirement ? 'runtime') OR jsonb_typeof(normalized_requirement -> 'runtime') = 'object')
        AND (NOT (normalized_requirement ? 'network') OR jsonb_typeof(normalized_requirement -> 'network') = 'object')
        AND (NOT (normalized_requirement ? 'integrations') OR (
            jsonb_typeof(normalized_requirement -> 'integrations') = 'array'
            AND jsonb_array_length(normalized_requirement -> 'integrations') <= 64
        ))
    ),
    PRIMARY KEY (tenant_id, artifact_version_id),
    FOREIGN KEY (tenant_id, artifact_version_id)
        REFERENCES public.artifact_descriptors (tenant_id, artifact_version_id)
);

CREATE FUNCTION public.enforce_artifact_requirement_metadata() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    descriptor_requirement jsonb;
BEGIN
    IF convert_from(NEW.normalized_bytes, 'UTF8')::jsonb IS DISTINCT FROM NEW.normalized_requirement THEN
        RAISE EXCEPTION 'requirement bytes and normalized requirement differ'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_requirements_metadata_check';
    END IF;

    SELECT normalized_metadata -> 'requirements'
      INTO descriptor_requirement
      FROM public.artifact_descriptors
     WHERE tenant_id = NEW.tenant_id
       AND artifact_version_id = NEW.artifact_version_id;

    IF NOT FOUND OR descriptor_requirement IS DISTINCT FROM NEW.normalized_requirement THEN
        RAISE EXCEPTION 'requirement does not match parent descriptor'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_requirements_descriptor_check';
    END IF;
    RETURN NEW;
EXCEPTION
    WHEN character_not_in_repertoire OR invalid_text_representation THEN
        RAISE EXCEPTION 'requirement bytes are not valid UTF-8 JSON'
            USING ERRCODE = '23514', CONSTRAINT = 'artifact_requirements_metadata_check';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_artifact_requirement_metadata() FROM PUBLIC;

CREATE TRIGGER artifact_requirement_metadata_guard
BEFORE INSERT ON public.artifact_requirements
FOR EACH ROW EXECUTE FUNCTION public.enforce_artifact_requirement_metadata();

ALTER TABLE public.artifact_requirements ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifact_requirements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifact_requirements
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.reject_artifact_requirement_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact requirements are immutable after registration';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_artifact_requirement_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_requirement_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifact_requirements
FOR EACH ROW EXECUTE FUNCTION public.reject_artifact_requirement_mutation();
