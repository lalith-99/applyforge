# ApplyForge — Implementation Plan

Full phase definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) §72. Summary:

- [x] **Phase 0** — Architecture + repository scaffolding.
- [x] **Phase 1** — Database foundation, authentication, user profiles, job preferences, onboarding.
- [x] **Phase 2** — Master resume upload/storage/extraction/parsing, candidate skill profile, review UI.
- [x] **Phase 3** — Job ingestion (Greenhouse/Lever/Ashby), normalization, dedup, freshness, scheduler.
- [x] **Phase 4** — JD parsing, JobRequirements, skill normalization, requirements storage.
- [x] **Phase 5** — Matching engine: eligibility, deterministic scoring, transferable skills, Opportunity
      Score, Current vs Target Match, golden tests.
- [x] **Phase 6** — Jobs UI: filters, job cards, job detail, match explanations.
- [x] **Phase 7** — Resume tailoring: STRICT/GROWTH/MAX_MATCH, suggestions, diff UI, approvals, Resume
      Alignment (this delivery).
- [ ] **Phase 8** — Quick Prep, Defend This Bullet, Make Me Qualified, Interview Readiness, learning plans.
- [ ] **Phase 9** — Resume generation: PDF, DOCX, versioning, preview.
- [ ] **Phase 10** — Applications: tracking, Kanban, application answers, events.
- [ ] **Phase 11** — Analytics: conversion funnel, response rates, match-score analytics.
- [ ] **Phase 12** — Production hardening: security, observability, performance, deployment, CI/CD, docs.

Each phase is implemented independently, per the working style in MASTER_REQUIREMENTS.md §73: review the
repo and docs first, state assumptions, identify DB/API/test impact, implement, then run formatters, lint,
tests, and builds before updating documentation and stopping for review.

## Phase 0 — what was actually delivered

See [DECISIONS.md](DECISIONS.md) for the full list of concrete decisions and the root README for verified
commands. In summary: monorepo layout, all docs listed above, a Next.js (TypeScript, App Router, Tailwind)
scaffold, a Go module (`chi` router) with `/health` and `/ready`, a FastAPI scaffold with `/health` and
`/ready`, a Docker Compose stack (Postgres + api + ai-worker), a Makefile, `.env.example`, and a GitHub
Actions CI skeleton that lints/tests/builds all three services.

## Phase 1 — what was actually delivered

* **Database**: goose migrations for `users`, `sessions`, `user_profiles`, `job_preferences` (with the full
  granular immigration-preference fields from the master spec); sqlc-generated Go query code.
* **Auth** (`internal/auth`): email/password signup+login (bcrypt), Google OAuth (authorization-code flow,
  gracefully disabled if env vars are unset), opaque database-backed sessions via an HttpOnly cookie,
  `RequireAuth` middleware.
* **Profile & preferences** (`internal/profile`, `internal/preferences`): upsert-based GET/PATCH APIs backed
  by the new tables.
* **Frontend**: signup/login pages, a two-step onboarding wizard (personal & career, then job
  preferences/restrictions/immigration), a minimal post-onboarding dashboard placeholder, a fetch-based API
  client, and zod-validated forms via react-hook-form.
* **Tests**: Go unit tests for password hashing, session tokens, and the auth service (via fakes); Go
  integration tests for all four repositories against a real Postgres instance (transaction-rolled-back, via
  `internal/testdb`), skipped automatically when `DATABASE_URL` is unset. CI now runs a Postgres service and
  goose migrations before `go test`.
* Full manual end-to-end verification via curl against the docker-compose stack: signup, duplicate-email
  rejection, login, wrong-password rejection, session lookup, profile/preferences upsert (including
  independent `requires_h1b_transfer` vs `requires_new_h1b_cap_sponsorship` tracking), logout, and
  post-logout 401s.

## Phase 2 — what was actually delivered

* **Storage**: MinIO added to docker-compose (S3-compatible); `internal/storage` (minio-go client),
  auto-creates the bucket on startup.
* **Background jobs**: `background_jobs` table + `internal/background` (SELECT...FOR UPDATE SKIP LOCKED,
  bounded exponential backoff, dead-letter after max_attempts). Runs as an in-process worker goroutine
  inside `cmd/api` (simplified from the suggested separate `cmd/worker` binary — see DECISIONS.md).
* **AI worker**: `POST /v1/resumes/extract` (PyMuPDF/python-docx text extraction) and
  `POST /v1/resumes/parse` (heuristic structured `ResumeProfile` — regex/keyword based, not a real LLM; see
  AI_PIPELINE.md).
* **Go**: `internal/resume` (upload/list/get, MIME+size validation, parse-job enqueue),
  `internal/candidateskills` (first-class CandidateSkill model), `internal/skills` (alias normalization).
* **Frontend**: `/resume` page — upload, status polling, parsed profile review (skills, experience,
  education).
* Verified end-to-end via docker-compose: upload → extract → parse → candidate_skills materialized, with a
  real generated PDF fixture.

## Phase 3 — what was actually delivered

* **DB**: `companies`, `job_sources`, `jobs`; seed migration for 3 real public boards (Greenhouse:
  `robinhood`, Lever: `lever`, Ashby: `ramp`).
* **Go**: `internal/jobs` — `JobSource` interface + Greenhouse/Lever/Ashby connectors (real public APIs, no
  scraping), normalization (title/company), idempotent upsert dedup (`source+external_id`, `content_hash`),
  `internal/scheduler` (hourly ticker, configurable via `JOB_POLL_INTERVAL_MINUTES`), an admin manual-sync
  endpoint, `GET /jobs` / `GET /jobs/{id}`.
* **Tests**: connector contract tests via `httptest` fixtures (no live network in CI), normalization unit
  tests, dedup/upsert-idempotency integration tests.
* Verified live: real ingestion pulled 128 Greenhouse + 142 Ashby jobs on first sync.

## Phase 4 — what was actually delivered

* **DB**: `job_requirements`, `skill_aliases` (seeded with common aliases).
* **AI worker**: `POST /v1/jobs/parse-requirements` (heuristic required-vs-preferred skill/section
  detection, seniority/experience-years/education/clearance/work-authorization extraction).
* **Go**: `internal/jobrequirements` — caches parsed requirements per `content_hash` (`GetOrParse`), so JD
  parsing runs at most once per unique job content, per the cost-management principle in §47.
* Verified live against real ingested job descriptions, including cache-hit verification (`parsed_at`
  unchanged across repeated requests).

## Phase 5 — what was actually delivered

* **DB**: `transferable_skills` (seeded with the exact pairs from MASTER_REQUIREMENTS.md §24),
  `job_matches`.
* **Go**: `internal/matching` — pure, DB-free deterministic scorer (`Score(Input) Result`), eligibility hard
  filters, transferable-skill partial credit (capped below direct-match credit), Opportunity Score,
  Current vs Target Profile Match. `internal/matching/service.go` wires it to real candidate/job/preference
  data and caches results in `job_matches`. `GET /jobs/{id}/match`.
* **Golden tests** (§55): Go/Kafka candidate scores strongly against a Go/Kafka role; Java candidate scores
  strongly against Java/Spring; React candidate scores low against a Go backend role; Kafka→SQS and
  PostgreSQL→DynamoDB register as transferable but never equal direct-skill credit; small wording changes
  don't swing scores; excluded-company hard failure zeroes the Opportunity Score.
* Verified live: real jobs scored correctly (Spark-requiring job → 55/Poor for a Go/Kafka resume; a real Go
  job → 85/Strong).

## Phase 6 — what was actually delivered

* **Frontend**: `/jobs` (search/remote-type/employment-type/posted-within/sort filters, paginated list),
  `JobCard` (lazily fetches its own match score client-side, matched-skill chips, age badge), `/jobs/[id]`
  detail page (match breakdown, current/target profile match, matched/transferable/missing skills,
  responsibilities, full description).
* Verified via production build + smoke test (200 responses for list and detail pages).

## Phase 7 — what was actually delivered

* **AI worker**: `POST /v1/tailoring/suggest` — heuristic STRICT/GROWTH/MAX_MATCH suggestion generation
  (STRICT never proposes new skills; GROWTH only proposes skills with transferable-skill support; MAX_MATCH
  proposes all missing required/preferred skills, with lower confidence/higher risk when there's no
  transfer basis).
* **Go**: `internal/tailoring` — `ComputeAlignment` (deterministic Resume Alignment Score, distinct from Job
  Match Score, never called an "ATS score"), `tailoring_runs`/`tailoring_suggestions` persistence,
  `POST /jobs/{id}/tailor`, `GET /tailoring/{id}`, `PATCH /tailoring/{id}/suggestions/{id}`,
  `POST /tailoring/{id}/approve-all`.
* **Frontend**: `/jobs/[id]/tailor` — resume/mode selection, suggestion cards with Approve/Reject, "AI
  Suggested" badges, Approve All Selected, alignment before/after.
* Verified live end-to-end: tailoring run created, summary + experience suggestions generated, approve-all
  flips all suggestions to APPROVED.

## Next up: Phase 8 (not started)

Quick Prep, Defend This Bullet, Make Me Qualified, Interview Readiness, learning plans. Do not begin this
until explicitly requested.

## Known scope limitations after Phases 2-7 (see DECISIONS.md for full list)

* No real AI provider integration yet — resume parsing, JD parsing, and tailoring all use deterministic
  heuristic stand-ins behind the same interface shape a real `AIProvider` would use. This was a deliberate,
  documented scope decision (no `AI_API_KEY` available), not an oversight.
* The Immigration-Aware Job Matching sub-system (DOL data ingestion, `ImmigrationCompatibility`,
  immigration-weighted Opportunity Score) is not implemented — it's a large sub-system of its own and was
  explicitly out of scope for "Phases 2-7" as enumerated in §72.
* Education/certification cross-referencing in match scoring is a flat partial-credit default, not a real
  comparison against parsed resume education/certifications.
* Domain alignment in the match score uses a flat default (no domain extraction in the heuristic JD parser).

