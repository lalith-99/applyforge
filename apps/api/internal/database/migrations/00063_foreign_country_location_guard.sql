-- +goose Up
-- Prevent explicit India locations from remaining in the U.S.-only catalog
-- because a provider-supplied region/country code such as "IN" was mistaken
-- for the U.S. state abbreviation for Indiana.
--
-- Future ingestion is protected in normalizeLocation; this migration repairs
-- already-ingested rows so the default country=US catalog filter stops
-- surfacing them immediately after upgrade.
UPDATE jobs
SET country_code = 'IN',
    state_code = NULL,
    eligible_country_codes = ARRAY['IN'],
    location_confidence = 'HIGH',
    remote_scope = CASE
        WHEN workplace_type = 'REMOTE' THEN 'UNKNOWN'
        ELSE remote_scope
    END,
    updated_at = now()
WHERE country_code = 'US'
  AND (
      lower(trim(COALESCE(country, ''))) IN ('india', 'in', 'ind')
      OR (
          trim(COALESCE(country, '')) = ''
          AND lower(trim(COALESCE(location_text, ''))) ~ '(^|,[[:space:]]*)india$'
      )
  );

-- +goose Down
-- Data-quality repair is intentionally not reversed.
