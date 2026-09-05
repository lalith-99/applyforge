-- name: GetSkillAliasesAsMap :many
SELECT alias, canonical_name FROM skill_aliases;

-- name: GetTransferableSkill :one
SELECT * FROM transferable_skills
WHERE lower(source_skill) = lower($1) AND lower(target_skill) = lower($2);

-- name: ListTransferableSkillsFromSources :many
SELECT * FROM transferable_skills WHERE lower(source_skill) = ANY($1::text[]);
