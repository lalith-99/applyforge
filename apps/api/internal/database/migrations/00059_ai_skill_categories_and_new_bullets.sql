-- +goose Up
ALTER TABLE tailoring_suggestions
    ADD COLUMN skill_categories JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN operation TEXT NOT NULL DEFAULT 'REWRITE',
    ADD COLUMN target_company TEXT,
    ADD COLUMN target_title TEXT;

ALTER TABLE tailoring_suggestions
    ADD CONSTRAINT tailoring_suggestions_operation_check
    CHECK (operation IN ('REWRITE', 'ADD'));

-- +goose Down
ALTER TABLE tailoring_suggestions
    DROP CONSTRAINT IF EXISTS tailoring_suggestions_operation_check,
    DROP COLUMN IF EXISTS target_title,
    DROP COLUMN IF EXISTS target_company,
    DROP COLUMN IF EXISTS operation,
    DROP COLUMN IF EXISTS skill_categories;
