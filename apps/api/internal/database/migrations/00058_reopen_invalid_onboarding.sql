-- +goose Up
-- Older onboarding accepted empty ranking-critical career fields and still
-- marked the profile complete. Reopen only those invalid rows so the user can
-- supply the fields required by recommendation ranking.
UPDATE user_profiles
SET onboarding_completed_at = NULL,
    updated_at = now()
WHERE onboarding_completed_at IS NOT NULL
  AND (
      cardinality(primary_target_titles) = 0
      OR seniority IS NULL
      OR btrim(seniority) = ''
      OR years_experience IS NULL
      OR years_experience <= 0
  );

-- +goose Down
-- Data repair is intentionally not reversible: restoring an invalid
-- completion marker would recreate the broken onboarding state.
SELECT 1;
