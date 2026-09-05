-- name: UpsertLearningPlan :one
INSERT INTO learning_plans (
    user_id, job_id, skills, current_readiness, target_readiness, topics,
    practice_questions, projects, architecture_questions, estimated_effort_category
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id, job_id) DO UPDATE SET
    skills = EXCLUDED.skills,
    current_readiness = EXCLUDED.current_readiness,
    target_readiness = EXCLUDED.target_readiness,
    topics = EXCLUDED.topics,
    practice_questions = EXCLUDED.practice_questions,
    projects = EXCLUDED.projects,
    architecture_questions = EXCLUDED.architecture_questions,
    estimated_effort_category = EXCLUDED.estimated_effort_category
RETURNING *;

-- name: GetLearningPlan :one
SELECT * FROM learning_plans WHERE user_id = $1 AND job_id = $2;
