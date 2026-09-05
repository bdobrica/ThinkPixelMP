CREATE FUNCTION public.reject_artifact_version_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'artifact versions are immutable after registration';
END;
$$;

REVOKE EXECUTE ON FUNCTION public.reject_artifact_version_mutation() FROM PUBLIC;

CREATE TRIGGER artifact_version_mutation_guard
BEFORE UPDATE OR DELETE ON public.artifact_versions
FOR EACH ROW EXECUTE FUNCTION public.reject_artifact_version_mutation();
