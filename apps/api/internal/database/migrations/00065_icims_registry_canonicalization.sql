-- +goose Up
-- iCIMS job/apply URLs identify individual requisitions, not distinct ATS
-- sources. Historically company_source_registry included source_url in its
-- identity, so every observed /jobs/<id>/... URL could create another row for
-- the same company + iCIMS tenant. Canonicalize iCIMS registry entries to one
-- tenant root per company and enforce that invariant at the database boundary.

-- Inspection payloads embed registry UUIDs. Drop only active iCIMS inspection
-- work before replacing duplicate registry rows; the scheduler will enqueue a
-- fresh inspection for each surviving canonical tenant. Historical completed
-- and dead-letter jobs remain untouched for diagnostics.
DELETE FROM background_jobs
WHERE job_type = 'inspect_company_source'
  AND status IN ('PENDING', 'RUNNING')
  AND upper(COALESCE(payload->>'source_type', '')) = 'ICIMS';

CREATE TEMP TABLE tmp_icims_registry_canonical
ON COMMIT DROP
AS
WITH grouped AS (
    SELECT
        csr.company_id,
        lower(trim(csr.board_token)) AS board_token,
        bool_or(csr.monitorable) OR bool_or(js.id IS NOT NULL) AS monitorable,
        max(csr.confidence) AS confidence,
        min(csr.first_seen_at) AS first_seen_at,
        max(csr.last_seen_at) AS last_seen_at,
        max(csr.last_verified_at) AS last_verified_at,
        max(csr.last_inspection_at) AS last_inspection_at,
        bool_or(csr.discovery_method = 'MANUAL') AS had_manual_discovery
    FROM company_source_registry csr
    LEFT JOIN job_sources js
      ON js.company_id = csr.company_id
     AND js.source_type = 'ICIMS'
     AND lower(trim(js.board_token)) = lower(trim(csr.board_token))
     AND js.enabled = true
    WHERE csr.source_type = 'ICIMS'
      AND trim(csr.board_token) <> ''
    GROUP BY csr.company_id, lower(trim(csr.board_token))
)
SELECT
    company_id,
    board_token,
    'https://' || board_token AS source_url,
    CASE
        WHEN monitorable THEN 'CAREER_PAGE'
        WHEN had_manual_discovery THEN 'MANUAL'
        ELSE 'JOB_URL'
    END AS discovery_method,
    confidence,
    monitorable,
    first_seen_at,
    last_seen_at,
    last_verified_at,
    last_inspection_at
FROM grouped;

DELETE FROM company_source_registry
WHERE source_type = 'ICIMS'
  AND trim(board_token) <> '';

INSERT INTO company_source_registry (
    company_id,
    source_type,
    board_token,
    source_url,
    discovery_method,
    confidence,
    monitorable,
    first_seen_at,
    last_seen_at,
    last_verified_at,
    last_error,
    inspection_status,
    inspection_attempt_count,
    next_inspection_at,
    last_inspection_at,
    inspection_last_error
)
SELECT
    company_id,
    'ICIMS',
    board_token,
    source_url,
    discovery_method,
    confidence,
    monitorable,
    first_seen_at,
    last_seen_at,
    last_verified_at,
    NULL,
    CASE WHEN monitorable THEN 'RESOLVED' ELSE 'PENDING' END,
    0,
    CASE WHEN monitorable THEN NULL ELSE now() END,
    last_inspection_at,
    NULL
FROM tmp_icims_registry_canonical;

-- Canonicalize future writes before uniqueness checks. This keeps old callers,
-- broad-feed ingestion, inspection promotion, and concurrent workers from
-- creating per-requisition iCIMS registry identities again.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION canonicalize_icims_registry_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.source_type = 'ICIMS' AND trim(NEW.board_token) <> '' THEN
        NEW.board_token := lower(trim(NEW.board_token));
        NEW.source_url := 'https://' || NEW.board_token;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER company_source_registry_icims_canonicalize
BEFORE INSERT OR UPDATE OF source_type, board_token, source_url
ON company_source_registry
FOR EACH ROW
EXECUTE FUNCTION canonicalize_icims_registry_identity();

-- The existing four-column identity index remains useful for all source types.
-- This narrower invariant documents and enforces iCIMS's tenant-level model.
CREATE UNIQUE INDEX company_source_registry_icims_tenant_idx
    ON company_source_registry (company_id, board_token)
    WHERE source_type = 'ICIMS';

-- +goose Down
DROP INDEX IF EXISTS company_source_registry_icims_tenant_idx;
DROP TRIGGER IF EXISTS company_source_registry_icims_canonicalize ON company_source_registry;
DROP FUNCTION IF EXISTS canonicalize_icims_registry_identity();

-- Historical per-requisition duplicates intentionally are not recreated.
