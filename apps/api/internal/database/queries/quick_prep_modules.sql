-- name: GetQuickPrepModule :one
SELECT * FROM quick_prep_modules WHERE normalized_skill = $1;

-- name: UpsertQuickPrepModule :one
INSERT INTO quick_prep_modules (
    normalized_skill, what_it_is, why_it_matters, transferable_from, core_concepts,
    screening_points, interview_questions, common_mistakes, architecture_questions, example_code
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (normalized_skill) DO UPDATE SET
    what_it_is = EXCLUDED.what_it_is,
    why_it_matters = EXCLUDED.why_it_matters,
    transferable_from = EXCLUDED.transferable_from,
    core_concepts = EXCLUDED.core_concepts,
    screening_points = EXCLUDED.screening_points,
    interview_questions = EXCLUDED.interview_questions,
    common_mistakes = EXCLUDED.common_mistakes,
    architecture_questions = EXCLUDED.architecture_questions,
    example_code = EXCLUDED.example_code,
    generated_at = now()
RETURNING *;
