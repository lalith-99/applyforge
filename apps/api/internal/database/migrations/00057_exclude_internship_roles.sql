-- +goose Up
-- The single-user MVP targets experienced full-time software roles.
-- Clean up already-ingested internship/student postings immediately; future
-- polls use the same title exclusions in classifyTitle.
UPDATE jobs
SET job_family = 'EXCLUDED',
    role_classification = 'NON_SOFTWARE',
    role_classification_confidence = 0.99,
    updated_at = now()
WHERE role_classification = 'IC_SOFTWARE'
  AND lower(title) ~ '(^|[^a-z])(intern(ship)?|co[- ]?op|apprentice(ship)?|student)([^a-z]|$)';

-- +goose Down
-- Data-only cleanup is intentionally irreversible: restoring these rows to
-- IC_SOFTWARE would require re-running the classifier on each title.
SELECT 1;
