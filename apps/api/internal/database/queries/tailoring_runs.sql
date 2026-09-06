-- name: CreateTailoringRun :one
INSERT INTO tailoring_runs (user_id, job_id, resume_id, mode, alignment_score_before)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CompleteTailoringRun :one
UPDATE tailoring_runs
SET status = 'COMPLETED', summary_suggestion = $2, keyword_coverage = $3, alignment_score_after = $4, completed_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateTailoringRunStatus :exec
-- Advances the run through intermediate stages (WRITING/EVALUATING/
-- REVISING) for a polling UI - CompleteTailoringRun/FailTailoringRun handle
-- the two terminal states.
UPDATE tailoring_runs SET status = $2 WHERE id = $1;

-- name: SetTailoringRunCritic :exec
UPDATE tailoring_runs SET critic_result = $2, revision_count = $3 WHERE id = $1;

-- name: FailTailoringRun :exec
UPDATE tailoring_runs SET status = 'FAILED', completed_at = now() WHERE id = $1;

-- name: GetTailoringRun :one
SELECT * FROM tailoring_runs WHERE id = $1;

-- name: GetTailoringRunForUser :one
SELECT * FROM tailoring_runs WHERE id = $1 AND user_id = $2;

-- name: ListTailoringRunsForJob :many
SELECT * FROM tailoring_runs WHERE user_id = $1 AND job_id = $2 ORDER BY created_at DESC;
