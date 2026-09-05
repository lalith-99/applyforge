-- name: CreateResumeExperience :one
INSERT INTO resume_experiences
    (resume_id, display_order, company, title, start_date, end_date, location, bullets, detected_skills, technologies)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListResumeExperiences :many
SELECT * FROM resume_experiences WHERE resume_id = $1 ORDER BY display_order ASC;

-- name: DeleteResumeExperiences :exec
DELETE FROM resume_experiences WHERE resume_id = $1;
