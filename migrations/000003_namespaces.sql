CREATE TABLE public.namespaces (
    tenant_id uuid NOT NULL,
    namespace_id uuid NOT NULL CHECK (uuid_extract_version(namespace_id) IS NOT DISTINCT FROM 7),
    path text NOT NULL CHECK (
        octet_length(path) BETWEEN 1 AND 255
        AND path ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$'
    ),
    owner_publisher_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, namespace_id),
    UNIQUE (tenant_id, path),
    FOREIGN KEY (tenant_id, owner_publisher_id)
        REFERENCES public.publishers (tenant_id, publisher_id)
);

CREATE FUNCTION public.enforce_namespace_verified_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1
      FROM public.publishers p
      JOIN public.publisher_state_records s
        ON s.tenant_id = p.tenant_id
       AND s.publisher_id = p.publisher_id
       AND s.version = p.current_state_version
     WHERE p.tenant_id = NEW.tenant_id
       AND p.publisher_id = NEW.owner_publisher_id
       AND s.state = 'verified'
       FOR SHARE OF p;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'namespace owner must be a verified tenant publisher'
            USING ERRCODE = '23514', CONSTRAINT = 'namespaces_verified_owner_check';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.enforce_namespace_verified_owner() FROM PUBLIC;

CREATE TRIGGER namespace_verified_owner_insert_guard
BEFORE INSERT ON public.namespaces
FOR EACH ROW EXECUTE FUNCTION public.enforce_namespace_verified_owner();

CREATE FUNCTION public.reject_namespace_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'namespace identity and ownership attribution are immutable';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_namespace_mutation() FROM PUBLIC;

CREATE TRIGGER namespace_mutation_guard
BEFORE UPDATE OR DELETE ON public.namespaces
FOR EACH ROW EXECUTE FUNCTION public.reject_namespace_mutation();

ALTER TABLE public.namespaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.namespaces FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON public.namespaces
    USING (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('thinkpixelmp.tenant_id', true), '')::uuid);
