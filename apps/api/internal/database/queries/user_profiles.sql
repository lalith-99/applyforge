-- name: UpsertUserProfile :one
INSERT INTO user_profiles (
    user_id, first_name, last_name, city, state, country,
    primary_target_titles, alternative_target_titles, seniority, years_experience,
    preferred_industries, preferred_technologies,
    desired_compensation_min, desired_compensation_max, desired_compensation_currency,
    onboarding_completed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
ON CONFLICT (user_id) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    city = EXCLUDED.city,
    state = EXCLUDED.state,
    country = EXCLUDED.country,
    primary_target_titles = EXCLUDED.primary_target_titles,
    alternative_target_titles = EXCLUDED.alternative_target_titles,
    seniority = EXCLUDED.seniority,
    years_experience = EXCLUDED.years_experience,
    preferred_industries = EXCLUDED.preferred_industries,
    preferred_technologies = EXCLUDED.preferred_technologies,
    desired_compensation_min = EXCLUDED.desired_compensation_min,
    desired_compensation_max = EXCLUDED.desired_compensation_max,
    desired_compensation_currency = EXCLUDED.desired_compensation_currency,
    onboarding_completed_at = COALESCE(EXCLUDED.onboarding_completed_at, user_profiles.onboarding_completed_at),
    updated_at = now()
RETURNING *;

-- name: GetUserProfile :one
SELECT * FROM user_profiles WHERE user_id = $1;
