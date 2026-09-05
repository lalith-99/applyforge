# ApplyForge — Matching Engine

## Principle

Job Match Score is always **deterministic**, computed in Go. An LLM is never asked to output a 0–100 score
directly. AI is only used upstream (JD parsing, transferable-skill reasoning) to produce structured inputs
that the deterministic scorer consumes.

## Job Match Score (planned, Phase 5)

| Component                  | Weight |
|-----------------------------|-------:|
| Must-have skill coverage    | 30 |
| Responsibility alignment    | 20 |
| Role/seniority              | 15 |
| Preferred skills             | 10 |
| Domain alignment             | 10 |
| Location/work arrangement    | 5 |
| Education/certifications     | 5 |
| Candidate preferences        | 5 |

Grades: 95–100 Exceptional, 90–94 Excellent, 80–89 Strong, 70–79 Possible, 60–69 Weak, <60 Poor.
Thresholds are configurable, not hardcoded.

## Related, distinct scores

* **Opportunity Score** — 75% Match + 15% Freshness + 10% Candidate Preferences. Eligibility hard failure
  overrides it.
* **Current vs Target Profile Match** — current = master-resume/confirmed skills only; target = includes
  user-approved learning/target skills. Target is never presented as verified current capability.
* **Resume Alignment Score** — how well a *specific resume* reflects the JD's requirements (keyword/
  responsibility/stack/domain coverage). Explicitly not called an "ATS score" and never expressed as a
  pass-probability.
* **Immigration Score** — separate 0–100 ranking signal for H-1B/green-card compatibility; never merged into
  the technical match score. For H-1B-transfer users, Opportunity Score reweights to 60% Match / 20%
  Immigration / 10% Freshness / 10% Preferences, but explicit immigration incompatibility is a hard
  ineligibility regardless of score.

## Eligibility

`EligibilityResult { eligible, hard_failures[], warnings[] }` computed before scoring. Ambiguous requirements
become warnings, not automatic failures. Immigration incompatibility (`requires_h1b_transfer=true` + explicit
"no sponsorship now or in the future") is a hard failure (`IMMIGRATION_INCOMPATIBLE`).

## Transferable Skill Engine

Computes conceptual distance between an existing skill and a requested one (e.g. Kafka → SQS is MEDIUM/HIGH,
never treated as identical). Returns `source_skill, target_skill, transferability_score, level
(VERY_HIGH|HIGH|MEDIUM|LOW|NONE), shared_concepts[], new_concepts_required[], reason, prep_classification
(QUICK_PREP|STANDARD_PREP|DEEPER_GAP)`.

## Golden tests (Phase 5)

Fixture-driven regression tests assert directional correctness (e.g. a Go/Kafka candidate scores strongly
against a Go/Kafka role; a React candidate does not score highly against a Go backend role; Kafka gives
partial, not full, transfer credit toward SQS) rather than exact score values, so small wording changes don't
cause unstable swings.

## Status (through Phase 5)

Implemented in `internal/matching`: a pure, database-free deterministic scorer (`Score(Input) Result`) plus a
thin service layer that fetches real candidate/job/preference/transferable-skill data and caches results in
`job_matches`. Eligibility hard filters, the weighted Job Match Score, transferable-skill partial credit
(capped below direct-skill credit), Opportunity Score, and Current vs Target Profile Match are all
implemented per the design below. `GET /jobs/{id}/match` computes and caches a result per (job, user).

Golden tests (§55) cover: strong same-stack matches, weak cross-stack matches, transferable-vs-direct credit
differences for Kafka→SQS and PostgreSQL→DynamoDB, wording-change stability, and excluded-company hard
failures.

**Known limitations:** domain alignment and education/certification cross-referencing use flat default
partial credit (no domain extraction in the heuristic JD parser, no resume-education cross-reference yet) —
see IMPLEMENTATION_PLAN.md for the full list. The Immigration-Aware Job Matching sub-system (§ in
MASTER_REQUIREMENTS.md) is not implemented.
