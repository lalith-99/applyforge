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

## Planned tables (introduced incrementally, not created in Phase 0)

users, user_profiles, job_preferences, candidate_skills, resumes, resume_versions, resume_experiences,
resume_facts, companies, company_aliases, job_sources, jobs, job_requirements, skill_aliases,
transferable_skills, job_matches, saved_jobs, tailoring_runs, tailoring_suggestions, learning_plans,
quick_prep_modules, applications, application_events, application_answers, background_jobs, ai_usage,
audit_events, employer_h1b_stats, employer_perm_stats, immigration_evidence, immigration_compatibility.

Full field-level definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) (§11, §12, §14, §17,
§18, §39, §48, §50, and the Immigration-Aware Job Matching section) and will be formalized as goose
migrations starting in Phase 1 (`users`, `user_profiles`, `job_preferences`) and Phase 2+ for the rest.

## Background job queue

`background_jobs` (Phase 1+) is the only queue in the MVP: `id, job_type, payload, status, attempts,
max_attempts, available_at, locked_at, locked_by, last_error, created_at, completed_at`. Workers claim rows
with `SELECT ... FOR UPDATE SKIP LOCKED`, supporting retry with bounded exponential backoff, a dead-letter
status, idempotency keys, timeouts, and graceful shutdown.

## Phase 0 status

No application schema exists yet. `docker-compose.yml` provisions an empty `postgres:16` instance with a
health check; the Go API only opens a pool and answers `/ready` by pinging the database.
