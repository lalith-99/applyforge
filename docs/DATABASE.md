# ApplyForge — Database

## Engine

PostgreSQL (Neon in production, Docker Compose `postgres:16` locally). Go owns the schema via `goose`
migrations; queries are generated with `sqlc`. Python does not write to Postgres directly in the MVP — it
receives/returns structured JSON to/from the Go API.

## Conventions

* Primary keys: UUID (`gen_random_uuid()` / `uuid_generate_v4()`).
* Timestamps: `timestamptz`, always UTC.
* Every table has `created_at`; mutable tables also have `updated_at`.
* Foreign keys enforced; cascade rules chosen per-table (documented at migration time).
* Every user-owned table indexed on `user_id`; every job/company table indexed on lookup columns
  (`posted_at`, `company_name`, `normalized_title`, `source`, `location`, `employment_type`).

## Implemented tables (through Phase 7)

**Phase 1 — auth & onboarding**

* **users** — `id, email, password_hash, google_id, email_verified_at, created_at, updated_at`. Either
  `password_hash` or `google_id` must be set (`users_password_or_google` check constraint). Email uniqueness
  is case-insensitive (`users_email_lower_idx`).
* **sessions** — opaque, database-backed session tokens: `id, user_id, token_hash, created_at, expires_at,
  last_used_at, user_agent, ip_address`. Only the SHA-256 hash of the raw token is stored; the raw token is
  the value of the `af_session` HttpOnly cookie. Default TTL: 30 days (`auth.SessionTTL`).
* **user_profiles** — 1:1 with `users`. Personal + career onboarding fields (§10).
* **job_preferences** — 1:1 with `users`. Work arrangement, employment type, salary floor, exclusions, and
  the granular immigration-preference fields (added at table-creation time rather than bolted on later).

**Phase 2 — resumes & candidate skills**

* **resumes** — `id, user_id, original_filename, mime_type, size_bytes, storage_key, status
  (UPLOADED/PARSING/PARSED/FAILED), parse_error, raw_text, parsed_profile (jsonb), parsed_at, created_at,
  updated_at`.
* **resume_experiences** — structured per-job entries parsed from a resume: `company, title, start_date,
  end_date, bullets[], detected_skills[], technologies[]`.
* **candidate_skills** — first-class CandidateSkill (§12): `normalized_name, display_name, category,
  proficiency, source, status`, unique per `(user_id, normalized_name)`.
* **background_jobs** — the Postgres-backed job queue (§48): `job_type, payload, status, attempts,
  max_attempts, available_at, locked_at, locked_by, last_error`.

**Phase 3 — job ingestion**

* **companies**, **job_sources** (`source_type, board_token, enabled, last_polled_at`), **jobs** (canonical
  job model, §14) — unique on `(source, external_id)`, indexed on `posted_at`, `normalized_title`,
  `company_id`, `status`.

**Phase 4 — JD parsing**

* **job_requirements** — structured JobRequirements cached per job, keyed by `content_hash` so re-parsing
  only happens when content actually changes.
* **skill_aliases** — deterministic alias → canonical-name mapping (§18), seeded with common aliases.

**Phase 5 — matching**

* **transferable_skills** — seeded conceptual-distance pairs (§24): `source_skill, target_skill,
  transferability_score, level, prep_classification`.
* **job_matches** — cached deterministic match results per `(job_id, user_id)`: scores, matched/missing/
  transferable skills, eligibility, Opportunity Score, Current/Target Profile Match.

**Phase 7 — resume tailoring**

* **tailoring_runs** — one per Tailor Resume request: `mode, status, alignment_score_before,
  alignment_score_after`.
* **tailoring_suggestions** — individual proposed changes: `section, original_text, suggested_text, source
  (MASTER_RESUME/AI_SUGGESTED), risk_level, user_status (PENDING/APPROVED/EDITED/REJECTED)`.

Migrations live in `apps/api/internal/database/migrations` (goose SQL format, numbered `00001`–`00019`).
sqlc queries live in `apps/api/internal/database/queries/*.sql`; generated code is committed at
`apps/api/internal/database/gen` (package `db`, `pgx/v5` driver, config in `apps/api/sqlc.yaml`).

## Planned tables (later phases)

resume_versions, resume_facts, company_aliases, saved_jobs, learning_plans, quick_prep_modules,
applications, application_events, application_answers, ai_usage, audit_events, employer_h1b_stats,
employer_perm_stats, immigration_evidence, immigration_compatibility.

Full field-level definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md).

## Background job queue

`background_jobs` is the only queue in the MVP: `id, job_type, payload, status, attempts, max_attempts,
available_at, locked_at, locked_by, last_error, created_at, completed_at`. Workers claim rows with
`SELECT ... FOR UPDATE SKIP LOCKED`, supporting retry with bounded exponential backoff, a dead-letter
status, idempotency keys, timeouts, and graceful shutdown.

## Testing

Repository integration tests (`internal/{users,profile,preferences,auth}/*_integration_test.go`) run against
a real Postgres instance via `internal/testdb`, inside a transaction that's rolled back after each test —
no seed/fixture cleanup needed. Tests skip automatically if `DATABASE_URL` is unset (e.g. a machine without
Postgres running); CI always sets it and runs goose migrations first.
