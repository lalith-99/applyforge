# ApplyForge — Implementation Plan

Full phase definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) §72. Summary:

- [x] **Phase 0** — Architecture + repository scaffolding.
- [x] **Phase 1** — Database foundation, authentication, user profiles, job preferences, onboarding (this delivery).
- [ ] **Phase 2** — Master resume upload/storage/extraction/parsing, candidate skill profile, review UI.
- [ ] **Phase 3** — Job ingestion (Greenhouse/Lever/Ashby), normalization, dedup, freshness, scheduler.
- [ ] **Phase 4** — JD parsing, JobRequirements, skill normalization, requirements storage.
- [ ] **Phase 5** — Matching engine: eligibility, deterministic scoring, transferable skills, Opportunity
      Score, Current vs Target Match, golden tests.
- [ ] **Phase 6** — Jobs UI: filters, job cards, job detail, match explanations.
- [ ] **Phase 7** — Resume tailoring: STRICT/GROWTH/MAX_MATCH, suggestions, diff UI, approvals, Resume
      Alignment.
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

## Next up: Phase 2 (not started)

Master resume upload, storage, PDF/DOCX text extraction, AI structured parsing, candidate skill profile, and
a resume review UI. Do not begin this until explicitly requested.
