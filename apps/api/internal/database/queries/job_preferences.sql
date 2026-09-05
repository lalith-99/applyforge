-- name: UpsertJobPreferences :one
INSERT INTO job_preferences (
    user_id, remote, hybrid, onsite, preferred_locations, willingness_to_relocate,
    employment_types, minimum_salary, excluded_companies, excluded_locations, excluded_industries,
    clearance_constraints, work_authorization,
    immigration_status, requires_h1b_transfer, requires_new_h1b_cap_sponsorship,
    requires_future_employment_sponsorship, green_card_support_preferred,
    green_card_support_required, perm_support_preferred, immigration_support_min_confidence
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)
ON CONFLICT (user_id) DO UPDATE SET
    remote = EXCLUDED.remote,
    hybrid = EXCLUDED.hybrid,
    onsite = EXCLUDED.onsite,
    preferred_locations = EXCLUDED.preferred_locations,
    willingness_to_relocate = EXCLUDED.willingness_to_relocate,
    employment_types = EXCLUDED.employment_types,
    minimum_salary = EXCLUDED.minimum_salary,
    excluded_companies = EXCLUDED.excluded_companies,
    excluded_locations = EXCLUDED.excluded_locations,
    excluded_industries = EXCLUDED.excluded_industries,
    clearance_constraints = EXCLUDED.clearance_constraints,
    work_authorization = EXCLUDED.work_authorization,
    immigration_status = EXCLUDED.immigration_status,
    requires_h1b_transfer = EXCLUDED.requires_h1b_transfer,
    requires_new_h1b_cap_sponsorship = EXCLUDED.requires_new_h1b_cap_sponsorship,
    requires_future_employment_sponsorship = EXCLUDED.requires_future_employment_sponsorship,
    green_card_support_preferred = EXCLUDED.green_card_support_preferred,
    green_card_support_required = EXCLUDED.green_card_support_required,
    perm_support_preferred = EXCLUDED.perm_support_preferred,
    immigration_support_min_confidence = EXCLUDED.immigration_support_min_confidence,
    updated_at = now()
RETURNING *;

-- name: GetJobPreferences :one
SELECT * FROM job_preferences WHERE user_id = $1;
