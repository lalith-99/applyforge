# ApplyForge — Implementation Plan

Full phase definitions live in [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) §72. Summary:

- [x] **Phase 0** — Architecture + repository scaffolding (this delivery).
- [ ] **Phase 1** — Database foundation, authentication, user profiles, job preferences, onboarding.
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

## Next up: Phase 1 (not started)

Database foundation (users, user_profiles, job_preferences via goose migrations + sqlc), authentication
(email + Google OAuth), profile and job-preference APIs, and the onboarding UI flow. Do not begin this until
explicitly requested.
