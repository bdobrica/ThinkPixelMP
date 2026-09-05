ALTER TABLE public.artifact_versions
    ADD CONSTRAINT artifact_versions_source_identity_key
    UNIQUE (tenant_id, artifact_version_id, resolved_digest, delivery_model);

CREATE TABLE public.artifact_sources (
    tenant_id uuid NOT NULL,
    artifact_version_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('oci', 'remote', 'import-record')),
    submitted_reference text CHECK (
        submitted_reference IS NULL
        OR octet_length(submitted_reference) BETWEEN 1 AND 2048
    ),
    resolved_reference text NOT NULL CHECK (octet_length(resolved_reference) BETWEEN 1 AND 2048),
    resolved_digest text NOT NULL CHECK (resolved_digest ~ '^sha256:[0-9a-f]{64}$'),
    endpoint text CHECK (
        endpoint IS NULL
        OR (octet_length(endpoint) BETWEEN 1 AND 2048 AND endpoint ~ '^https://[^[:space:]]+$')
    ),
    import_record_id uuid CHECK (
        import_record_id IS NULL
        OR uuid_extract_version(import_record_id) IS NOT DISTINCT FROM 7
    ),
    delivery_model text NOT NULL CHECK (delivery_model IN ('oci', 'remote', 'imported-source')),
    PRIMARY KEY (tenant_id, artifact_version_id),
    FOREIGN KEY (tenant_id, artifact_version_id, resolved_digest, delivery_model)
        REFERENCES public.artifact_versions (tenant_id, artifact_version_id, resolved_digest, delivery_model),
    CHECK (
        (kind = 'oci'
            AND delivery_model = 'oci'
            AND submitted_reference IS NOT NULL
            AND endpoint IS NULL
            AND import_record_id IS NULL
            AND resolved_reference LIKE '%@' || resolved_digest)
        OR (kind = 'remote'
            AND delivery_model = 'remote'
            AND submitted_reference IS NOT NULL
            AND endpoint IS NOT NULL
            AND import_record_id IS NULL)
        OR (kind = 'import-record'
            AND delivery_model = 'imported-source'
            AND submitted_reference IS NULL
            AND endpoint IS NULL
            AND import_record_id IS NOT NULL)
    )
);

ALTER TABLE public.artifact_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifact_sources FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifact_sources
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);

CREATE FUNCTION public.reject_artifact_source_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact sources are immutable after registration';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_artifact_source_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_source_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifact_sources
FOR EACH ROW EXECUTE FUNCTION public.reject_artifact_source_mutation();
