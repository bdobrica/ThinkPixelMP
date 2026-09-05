ALTER TABLE public.artifacts
    ADD CONSTRAINT artifacts_tenant_id_artifact_id_kind_key
    UNIQUE (tenant_id, artifact_id, kind);

CREATE TABLE public.artifact_versions (
    tenant_id uuid NOT NULL,
    artifact_version_id uuid NOT NULL CHECK (uuid_extract_version(artifact_version_id) IS NOT DISTINCT FROM 7),
    artifact_id uuid NOT NULL,
    publisher_id uuid NOT NULL,
    semantic_version text NOT NULL CHECK (
        octet_length(semantic_version) BETWEEN 5 AND 255
        AND semantic_version ~ '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$'
    ),
    resolved_digest text NOT NULL CHECK (resolved_digest ~ '^sha256:[0-9a-f]{64}$'),
    kind text NOT NULL CHECK (kind IN ('skill', 'agent-runtime', 'mcp-server', 'remote-agent', 'bundle')),
    artifact_class text NOT NULL CHECK (artifact_class IN ('instructional', 'executable-local', 'remote-service', 'composite')),
    delivery_model text NOT NULL CHECK (delivery_model IN ('oci', 'remote', 'imported-source')),
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'deprecated', 'quarantined', 'revoked')),
    registered_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, artifact_version_id),
    UNIQUE (tenant_id, artifact_id, semantic_version),
    UNIQUE (tenant_id, resolved_digest),
    FOREIGN KEY (tenant_id, artifact_id, kind)
        REFERENCES public.artifacts (tenant_id, artifact_id, kind),
    FOREIGN KEY (tenant_id, publisher_id)
        REFERENCES public.publishers (tenant_id, publisher_id),
    CHECK (
        delivery_model = 'imported-source'
        OR (kind = 'skill' AND artifact_class IN ('instructional', 'executable-local') AND delivery_model = 'oci')
        OR (kind = 'agent-runtime' AND artifact_class = 'executable-local' AND delivery_model = 'oci')
        OR (kind = 'mcp-server' AND (
            (artifact_class = 'executable-local' AND delivery_model = 'oci')
            OR (artifact_class = 'remote-service' AND delivery_model = 'remote')
        ))
        OR (kind = 'remote-agent' AND artifact_class = 'remote-service' AND delivery_model = 'oci')
        OR (kind = 'bundle' AND artifact_class = 'composite' AND delivery_model = 'oci')
    ),
    CHECK (
        delivery_model <> 'imported-source'
        OR (kind = 'skill' AND artifact_class IN ('instructional', 'executable-local'))
        OR (kind = 'agent-runtime' AND artifact_class = 'executable-local')
        OR (kind = 'mcp-server' AND artifact_class IN ('executable-local', 'remote-service'))
        OR (kind = 'remote-agent' AND artifact_class = 'remote-service')
        OR (kind = 'bundle' AND artifact_class = 'composite')
    )
);

ALTER TABLE public.artifact_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.artifact_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.artifact_versions
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);
