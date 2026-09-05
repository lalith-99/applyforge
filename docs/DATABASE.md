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

## Implemented tables (Phase 1)

* **users** — `id, email, password_hash, google_id, email_verified_at, created_at, updated_at`. Either
  `password_hash` or `google_id` must be set (`users_password_or_google` check constraint). Email uniqueness
  is case-insensitive (`users_email_lower_idx`).
* **sessions** — opaque, database-backed session tokens: `id, user_id, token_hash, created_at, expires_at,
  last_used_at, user_agent, ip_address`. Only the SHA-256 hash of the raw token is stored; the raw token is
  the value of the `af_session` HttpOnly cookie. Default TTL: 30 days (`auth.SessionTTL`).
* **user_profiles** — 1:1 with `users`. Personal + career onboarding fields (§10): name, location,
  target titles, seniority, years of experience, preferred industries/technologies, desired compensation,
  plus `onboarding_completed_at`.
* **job_preferences** — 1:1 with `users`. Work arrangement, employment type, salary floor, exclusions, and
  the granular immigration-preference fields from the Immigration-Aware Job Matching spec
  (`immigration_status`, `requires_h1b_transfer`, `requires_new_h1b_cap_sponsorship`,
  `requires_future_employment_sponsorship`, `green_card_support_preferred`, `green_card_support_required`,
  `perm_support_preferred`, `immigration_support_min_confidence`) — intentionally added now rather than
  bolted on later, per the spec's explicit instruction to extend `JobPreferences`.

Migrations live in `apps/api/internal/database/migrations` (goose SQL format, numbered `00001`–`00005`).
sqlc queries live in `apps/api/internal/database/queries/*.sql`; generated code is committed at
`apps/api/internal/database/gen` (package `db`, `pgx/v5` driver, config in `apps/api/sqlc.yaml`).

## Planned tables (later phases)

candidate_skills, resumes, resume_versions, resume_experiences, resume_facts, companies, company_aliases,
job_sources, jobs, job_requirements, skill_aliases, transferable_skills, job_matches, saved_jobs,
tailoring_runs, tailoring_suggestions, learning_plans, quick_prep_modules, applications,
application_events, application_answers, background_jobs, ai_usage, audit_events, employer_h1b_stats,
employer_perm_stats, immigration_evidence, immigration_compatibility.

Full field-level definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md).

## Background job queue

`background_jobs` (not yet created — introduced whichever phase first needs it, at the latest Phase 3's
scheduler) is the only queue in the MVP: `id, job_type, payload, status, attempts, max_attempts,
available_at, locked_at, locked_by, last_error, created_at, completed_at`. Workers claim rows with
`SELECT ... FOR UPDATE SKIP LOCKED`, supporting retry with bounded exponential backoff, a dead-letter
status, idempotency keys, timeouts, and graceful shutdown.

## Testing

Repository integration tests (`internal/{users,profile,preferences,auth}/*_integration_test.go`) run against
a real Postgres instance via `internal/testdb`, inside a transaction that's rolled back after each test —
no seed/fixture cleanup needed. Tests skip automatically if `DATABASE_URL` is unset (e.g. a machine without
Postgres running); CI always sets it and runs goose migrations first.
