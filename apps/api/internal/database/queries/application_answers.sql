-- name: UpsertApplicationAnswers :one
INSERT INTO application_answers (
    user_id, full_name, phone, email, location, desired_location, work_authorization,
    sponsorship, salary_expectation, notice_period, linkedin_url, github_url, portfolio_url, common_answers
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (user_id) DO UPDATE SET
    full_name = EXCLUDED.full_name,
    phone = EXCLUDED.phone,
    email = EXCLUDED.email,
    location = EXCLUDED.location,
    desired_location = EXCLUDED.desired_location,
    work_authorization = EXCLUDED.work_authorization,
    sponsorship = EXCLUDED.sponsorship,
    salary_expectation = EXCLUDED.salary_expectation,
    notice_period = EXCLUDED.notice_period,
    linkedin_url = EXCLUDED.linkedin_url,
    github_url = EXCLUDED.github_url,
    portfolio_url = EXCLUDED.portfolio_url,
    common_answers = EXCLUDED.common_answers,
    updated_at = now()
RETURNING *;

-- name: GetApplicationAnswers :one
SELECT * FROM application_answers WHERE user_id = $1;
