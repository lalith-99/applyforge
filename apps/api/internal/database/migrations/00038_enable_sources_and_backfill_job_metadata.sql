-- +goose Up
-- Enable the new direct ATS connector types and normalize metadata that
-- predates canonical employment/location/role-classification columns.

ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE'
    ));

-- Canonicalize common provider-specific employment labels so exact catalog
-- filtering behaves consistently for both existing and newly ingested rows.
UPDATE jobs
SET employment_type = CASE
    WHEN regexp_replace(lower(trim(employment_type)), '[ _/-]', '', 'g')
         IN ('full', 'fulltime', 'permanent', 'regular', 'regularfulltime', 'employee')
        THEN 'FullTime'
    WHEN regexp_replace(lower(trim(employment_type)), '[ _/-]', '', 'g')
         IN ('contract', 'contractor', 'freelance', 'consultant')
        THEN 'Contract'
    WHEN regexp_replace(lower(trim(employment_type)), '[ _/-]', '', 'g')
         IN ('intern', 'internship', 'studentintern')
        THEN 'Internship'
    WHEN regexp_replace(lower(trim(employment_type)), '[ _/-]', '', 'g')
         IN ('part', 'parttime')
        THEN 'PartTime'
    WHEN regexp_replace(lower(trim(employment_type)), '[ _/-]', '', 'g')
         IN ('temp', 'temporary', 'seasonal')
        THEN 'Temporary'
    ELSE employment_type
END
WHERE employment_type IS NOT NULL;

-- Safely recognize existing U.S. rows that end in a conventional uppercase
-- state abbreviation such as "Austin, TX". The regex is deliberately
-- case-sensitive so ordinary words such as "in", "or", "me", and "hi" are
-- never mistaken for Indiana/Oregon/Maine/Hawaii.
UPDATE jobs
SET country_code = 'US',
    eligible_country_codes = ARRAY['US'],
    location_confidence = 'HIGH'
WHERE country_code IS NULL
  AND location_text ~ '(^|,[[:space:]]*)(AL|AK|AZ|AR|CA|CO|CT|DE|FL|GA|HI|ID|IL|IN|IA|KS|KY|LA|ME|MD|MA|MI|MN|MS|MO|MT|NE|NV|NH|NJ|NM|NY|NC|ND|OH|OK|OR|PA|RI|SC|SD|TN|TX|UT|VT|VA|WA|WV|WI|WY|DC)(,|$)';

-- Backfill the deterministic title classifier for pre-existing rows. New
-- and subsequently refreshed rows are classified by internal/jobs/classify.go.
UPDATE jobs
SET
    job_family = CASE
        WHEN lower(title) ~ '(engineering manager|(^|[^a-z])manager([^a-z]|$)|(^|[^a-z])director([^a-z]|$)|vice president|(^|[^a-z])vp([^a-z]|$)|head of|quality assurance|(^|[^a-z])qa([^a-z]|$)|tester|test engineer|sdet|analyst|product manager|program manager|project manager|scrum master|support engineer|solutions engineer|sales engineer|mechanical engineer|civil engineer|electrical engineer|manufacturing engineer|industrial engineer|field engineer|process engineer|hardware engineer|quality engineer|validation engineer)'
            THEN 'EXCLUDED'
        WHEN lower(title) LIKE '%backend%' OR lower(title) LIKE '%back end%' THEN 'BACKEND'
        WHEN lower(title) LIKE '%frontend%' OR lower(title) LIKE '%front end%' THEN 'FRONTEND'
        WHEN lower(title) LIKE '%full stack%' OR lower(title) LIKE '%fullstack%' THEN 'FULLSTACK'
        WHEN lower(title) LIKE '%platform%' THEN 'PLATFORM'
        WHEN lower(title) LIKE '%infrastructure%' THEN 'INFRASTRUCTURE'
        WHEN lower(title) LIKE '%site reliability%' OR lower(title) ~ '(^|[^a-z])sre([^a-z]|$)' THEN 'SRE'
        WHEN lower(title) LIKE '%devops%' OR lower(title) LIKE '%devsecops%' THEN 'DEVOPS'
        WHEN lower(title) LIKE '%cloud engineer%' THEN 'CLOUD'
        WHEN lower(title) LIKE '%data engineer%' THEN 'DATA_ENGINEERING'
        WHEN lower(title) LIKE '%machine learning%' OR lower(title) ~ '(^|[^a-z])ml engineer' THEN 'ML_ENGINEERING'
        WHEN lower(title) ~ '(^|[^a-z])ai engineer' OR lower(title) LIKE '%artificial intelligence%' THEN 'AI_ENGINEERING'
        WHEN lower(title) LIKE '%security engineer%' OR lower(title) LIKE '%application security%' THEN 'SECURITY_ENGINEERING'
        WHEN lower(title) LIKE '%ios%' OR lower(title) LIKE '%android%' OR lower(title) LIKE '%mobile%' THEN 'MOBILE'
        WHEN lower(title) LIKE '%embedded%' OR lower(title) LIKE '%firmware%' THEN 'EMBEDDED'
        WHEN lower(title) LIKE '%systems software%' OR lower(title) LIKE '%systems engineer, software%' THEN 'SYSTEMS'
        WHEN lower(title) LIKE '%database engineer%' THEN 'INFRASTRUCTURE'
        WHEN lower(title) LIKE '%application engineer%' OR lower(title) LIKE '%application developer%' OR lower(title) LIKE '%web developer%' THEN 'SOFTWARE_ENGINEERING'
        WHEN lower(title) LIKE '%software engineer%' OR lower(title) LIKE '%software developer%' OR lower(title) LIKE '%software development engineer%' OR lower(title) ~ '(^|[^a-z])developer([^a-z]|$)' THEN 'SOFTWARE_ENGINEERING'
        ELSE job_family
    END,
    role_classification = CASE
        WHEN lower(title) ~ '(engineering manager|(^|[^a-z])manager([^a-z]|$)|(^|[^a-z])director([^a-z]|$)|vice president|(^|[^a-z])vp([^a-z]|$)|head of|quality assurance|(^|[^a-z])qa([^a-z]|$)|tester|test engineer|sdet|analyst|product manager|program manager|project manager|scrum master|support engineer|solutions engineer|sales engineer|mechanical engineer|civil engineer|electrical engineer|manufacturing engineer|industrial engineer|field engineer|process engineer|hardware engineer|quality engineer|validation engineer)'
            THEN 'NON_SOFTWARE'
        WHEN lower(title) LIKE '%backend%'
          OR lower(title) LIKE '%back end%'
          OR lower(title) LIKE '%frontend%'
          OR lower(title) LIKE '%front end%'
          OR lower(title) LIKE '%full stack%'
          OR lower(title) LIKE '%fullstack%'
          OR lower(title) LIKE '%platform%'
          OR lower(title) LIKE '%infrastructure%'
          OR lower(title) LIKE '%site reliability%'
          OR lower(title) ~ '(^|[^a-z])sre([^a-z]|$)'
          OR lower(title) LIKE '%devops%'
          OR lower(title) LIKE '%devsecops%'
          OR lower(title) LIKE '%cloud engineer%'
          OR lower(title) LIKE '%data engineer%'
          OR lower(title) LIKE '%machine learning%'
          OR lower(title) ~ '(^|[^a-z])ml engineer'
          OR lower(title) ~ '(^|[^a-z])ai engineer'
          OR lower(title) LIKE '%artificial intelligence%'
          OR lower(title) LIKE '%security engineer%'
          OR lower(title) LIKE '%application security%'
          OR lower(title) LIKE '%ios%'
          OR lower(title) LIKE '%android%'
          OR lower(title) LIKE '%mobile%'
          OR lower(title) LIKE '%embedded%'
          OR lower(title) LIKE '%firmware%'
          OR lower(title) LIKE '%systems software%'
          OR lower(title) LIKE '%systems engineer, software%'
          OR lower(title) LIKE '%database engineer%'
          OR lower(title) LIKE '%application engineer%'
          OR lower(title) LIKE '%application developer%'
          OR lower(title) LIKE '%web developer%'
          OR lower(title) LIKE '%software engineer%'
          OR lower(title) LIKE '%software developer%'
          OR lower(title) LIKE '%software development engineer%'
          OR lower(title) ~ '(^|[^a-z])developer([^a-z]|$)'
            THEN 'IC_SOFTWARE'
        ELSE role_classification
    END,
    role_classification_confidence = CASE
        WHEN lower(title) ~ '(engineering manager|(^|[^a-z])manager([^a-z]|$)|(^|[^a-z])director([^a-z]|$)|vice president|(^|[^a-z])vp([^a-z]|$)|head of|quality assurance|(^|[^a-z])qa([^a-z]|$)|tester|test engineer|sdet|analyst|product manager|program manager|project manager|scrum master|support engineer|solutions engineer|sales engineer|mechanical engineer|civil engineer|electrical engineer|manufacturing engineer|industrial engineer|field engineer|process engineer|hardware engineer|quality engineer|validation engineer)'
            THEN 0.98
        WHEN role_classification = 'UNKNOWN' THEN 0.90
        ELSE role_classification_confidence
    END
WHERE role_classification = 'UNKNOWN';

-- +goose Down
-- Metadata normalization/backfill is intentionally not reversed.
ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'ARBEITNOW'));
