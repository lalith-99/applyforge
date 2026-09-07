-- +goose Up
CREATE TABLE company_immigration_evidence (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employer_name             TEXT NOT NULL,
    employer_normalized_name  TEXT NOT NULL,
    program                   TEXT NOT NULL,
    fiscal_year               INTEGER NOT NULL,
    certified_count           INTEGER NOT NULL DEFAULT 0,
    denied_count              INTEGER NOT NULL DEFAULT 0,
    withdrawn_count           INTEGER NOT NULL DEFAULT 0,
    other_count               INTEGER NOT NULL DEFAULT 0,
    total_count               INTEGER NOT NULL DEFAULT 0,
    latest_decision_date      DATE,
    source_release            TEXT NOT NULL,
    source_url                TEXT NOT NULL,
    imported_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_immigration_evidence_program_check
        CHECK (program IN ('LCA_H1B', 'PERM')),
    CONSTRAINT company_immigration_evidence_counts_check
        CHECK (
            certified_count >= 0 AND
            denied_count >= 0 AND
            withdrawn_count >= 0 AND
            other_count >= 0 AND
            total_count >= 0
        ),
    UNIQUE (employer_normalized_name, program, fiscal_year, source_release)
);

CREATE INDEX company_immigration_evidence_employer_idx
    ON company_immigration_evidence (employer_normalized_name, fiscal_year DESC);
CREATE INDEX company_immigration_evidence_program_idx
    ON company_immigration_evidence (program, fiscal_year DESC);

CREATE TABLE company_immigration_aliases (
    company_id                         UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    evidence_employer_normalized_name  TEXT NOT NULL,
    match_method                       TEXT NOT NULL DEFAULT 'MANUAL',
    confidence                         REAL NOT NULL DEFAULT 1.0,
    created_at                         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, evidence_employer_normalized_name),
    CONSTRAINT company_immigration_aliases_method_check
        CHECK (match_method IN ('EXACT_NORMALIZED', 'MANUAL')),
    CONSTRAINT company_immigration_aliases_confidence_check
        CHECK (confidence >= 0 AND confidence <= 1)
);

-- Automatically link exact normalized employer names already present in the
-- catalog. Non-exact legal/division aliases remain explicit rather than fuzzy
-- guessed, so immigration history is never attached to the wrong employer.
INSERT INTO company_immigration_aliases (
    company_id,
    evidence_employer_normalized_name,
    match_method,
    confidence
)
SELECT
    c.id,
    c.normalized_name,
    'EXACT_NORMALIZED',
    1.0
FROM companies c
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE company_immigration_aliases;
DROP TABLE company_immigration_evidence;
