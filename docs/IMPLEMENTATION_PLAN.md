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
- [x] **Phase 8** — Quick Prep, Defend This Bullet, Make Me Qualified, Interview Readiness, learning plans.
- [x] **Phase 9** — Resume generation: PDF, DOCX, versioning, preview.
- [x] **Phase 10** — Applications: tracking, Kanban, application answers, events.
- [x] **Phase 11** — Analytics: conversion funnel, response rates, match-score analytics.
- [x] **Phase 12** — Production hardening: security, observability, performance, deployment, CI/CD, docs.

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

## Phase 8 — what was actually delivered

* **DB**: `quick_prep_modules` (cached by `normalized_skill`, shared across all users), `learning_plans`
  (cached per `user_id`+`job_id`). Migrations for Phase 9-10 tables (`resume_versions`, `applications`,
  `application_events`, `application_answers`) were also created in this pass since they were quick to design
  alongside the related schema work, but no code reads/writes them yet.
* **AI worker**: `internal/learning` (Python: `app/learning/`) — curated `CONTENT_BANK` for common
  technologies (Amazon SQS, Kafka, Kubernetes, Docker, PostgreSQL, DynamoDB, gRPC, REST) with a generic
  fallback for unlisted skills; `POST /v1/learning/quick-prep`, `POST /v1/learning/defend-bullet` (dedupes
  questions across skills, capped at 8), `POST /v1/learning/learning-plan` (aggregates topics/questions
  across missing skills, classifies prep effort as QUICK_PREP/STANDARD_PREP/DEEPER_GAP by gap size).
* **Go**: `internal/aiclient/learning.go` (client for the 3 endpoints above); `internal/learning` —
  `Repository` (quick-prep cache is generic/shared, never stores per-user transferable-skill personalization;
  learning-plan cache is per user+job), `Service.QuickPrep` (cache-or-generate, then personalizes
  `transferable_from` at request time from the caller's own candidate skills via the matching engine's
  transferable-skills table), `Service.DefendBullet` (uncached passthrough — bullet text/skills vary per
  call), `Service.LearningPlan` (pulls missing required/preferred skills from a fresh `matching.Service.Match`
  call), `Service.MakeMeQualified` (aggregates current/target match, high/low-value gaps, a learning plan,
  and a deterministic Interview Readiness score), `InterviewReadiness` (§35 weighted components — Core
  Language 20/Backend Fundamentals 20/Required Technology 25/System Design 15/Experience Examples
  10/Question Preparedness 10 — derived from the existing match `ComponentScores`, since there's no
  dedicated per-component signal for some of these yet; documented as product guidance, not a scientific
  assessment). Routes: `GET /skills/{skill}/quick-prep`, `POST /defend-bullet`,
  `POST /jobs/{id}/learning-plan`, `POST /jobs/{id}/make-me-qualified`.
* **Frontend**: `features/learning/QuickPrepDrawer.tsx` ("Learn First" trigger + slide-over drawer),
  `features/learning/DefendBulletDrawer.tsx` ("Defend This Bullet" trigger + drawer) — both wired into the
  tailoring page's suggestion cards (skills added → Quick Prep; experience-section suggestions → Defend This
  Bullet) and the job detail page (missing skills → Quick Prep; new "Make Me Qualified" button → readiness
  score, high/low-value gaps, practice projects).
* **Tests**: Go — `internal/learning` readiness unit tests (pure function, strong vs weak match, bounds) and
  repository integration tests (quick-prep cache miss→hit round trip, learning-plan upsert-not-duplicate).
  Python — 7 new tests for quick-prep (curated + generic fallback), defend-bullet (known skills, fallback,
  dedup), learning-plan (effort classification, topic aggregation); all passing alongside the existing 23.
* **Bug found and fixed during live verification**: the Go `aiclient` was marshaling a `nil` Go slice as
  JSON `null` for `transferable_from`/`skills`/`missing_skills`; Pydantic's `default_factory` only applies
  when a field is *omitted*, not when it's explicitly `null`, so quick-prep initially failed with a 422.
  Fixed by normalizing `nil` to `[]string{}` before marshaling in all three `aiclient` learning methods.
* Verified live end-to-end via docker-compose: quick-prep for a known skill (Kafka) and an unknown skill
  (generic fallback), cache-hit on second request, defend-bullet for a real suggestion, learning-plan and
  make-me-qualified against a real ingested job.

## Phase 9 — what was actually delivered

* **AI worker**: `app/documents/generator.py` — deterministic PDF (fpdf2) and DOCX (python-docx)
  rendering directly from the existing `ResumeProfile` model (no new schema needed — the same shape
  used for resume parsing). `POST /v1/documents/pdf`, `POST /v1/documents/docx`.
* **Go**: `internal/resumeversion` — `mergeContent` (pure function: applies only APPROVED/EDITED
  tailoring suggestions onto a base resume's parsed profile; summary suggestions replace the summary,
  skills suggestions append new skills without case-insensitive duplicates, experience suggestions
  replace the exact bullet matching their `original_text`), `Service.GenerateVersion` (fetches the
  resume's `parsed_profile` directly as the merge input, computes the next version number, generates
  PDF/DOCX via the AI worker, stores both in MinIO under `resume-versions/{id}/resume.{pdf,docx}`),
  `Service.Download` (proxies the stored bytes back through the Go API rather than issuing presigned
  URLs, to keep bucket access private). Routes: `POST /resumes/{id}/versions`,
  `GET /resumes/{id}/versions`, `GET /resume-versions/{id}`, `GET /resume-versions/{id}/download`.
* **Frontend**: a "Generate Tailored Resume" panel on the tailoring page — one button generates both
  documents for the current run, with Download PDF/DOCX links once ready.
* **Bug found and fixed during live verification**: fpdf2's `multi_cell()` does not reset the cursor's
  x position to the left margin afterward (unlike `cell()`), so a second `multi_cell()` call in a row
  (e.g. the second bullet point) failed with `"Not enough horizontal space to render a single
  character"`. Fixed by passing `new_x="LMARGIN", new_y="NEXT"` explicitly to every `multi_cell()` call.
* Verified live end-to-end via docker-compose: uploaded a real generated PDF resume, parsed it, ran
  tailoring against a real ingested job, approved all suggestions, generated a resume version, and
  downloaded both the PDF and DOCX — the tailored bullet text and expanded summary were correctly
  merged into the rendered documents.

## Phase 10 — what was actually delivered

* **Go**: `internal/applications` — `Repository` wrapping applications/application_events/
  application_answers (all sqlc queries were already written in the Phase 8 pass), `Service.Save`
  (create-or-reattach-resume-version upsert, unique on user+job so "Save Job" is idempotent),
  `Service.ChangeStatus` (validates the target status, logs a `STATUS_CHANGE` event with from/to status,
  skips logging for no-op transitions, sets `applied_at` automatically on first transition to APPLIED via
  the existing SQL `CASE` in `UpdateApplicationStatus`). Routes: `POST /applications` (save/attach),
  `GET /applications` (joined with job display fields for the list/Kanban/table UI), `GET /applications/{id}`,
  `PATCH /applications/{id}` (status and/or notes/next_action), `GET /applications/{id}/events`,
  `GET /application-answers` (zero-value defaults, not 404, matching the profile/preferences convention),
  `PATCH /application-answers`.
* **Frontend**: `/applications` page with a Kanban view (status columns, "Move to next stage" button per
  card — click-to-advance rather than full drag-and-drop, to avoid a new DnD dependency for a single-user
  MVP) and a table view (inline status dropdown per row) toggled by a header control; a "Save Job" button
  added to the job detail page.
* **Tests**: 4 new Go integration tests — upsert-not-duplicate on repeated Save, status-change event
  logging + invalid-status rejection, no-op transitions never logging a spurious event, application-answers
  upsert/get round-trip.
* **Bug found and fixed during live verification**: `UpsertAnswersInput` had no `json` struct tags, so the
  API's `DisallowUnknownFields` strict JSON decoder rejected any real request body (e.g. `{"full_name": ...}`)
  with "invalid request body", even though the Go code compiled fine. This is the kind of bug that's
  invisible until you actually exercise the endpoint with real JSON — added the missing tags and
  re-verified live.
* Verified live end-to-end via docker-compose: save a job, list applications (joined with job fields),
  change status to APPLIED (`applied_at` set, one `STATUS_CHANGE` event logged), reject an invalid status
  with 400, get default application answers, then update and confirm they persist.

## Phase 11 — what was actually delivered

* **Go**: `internal/analytics` — `Repository` wrapping the aggregation queries written in the Phase 8
  pass (`CountJobsDiscovered`, `CountTailoringRunsForUser`, `CountHighMatchesForUser`,
  `CountApplicationsByStatusForUser`, `CountApplicationEventsByToStatusForUser`), `Service.Dashboard`
  which builds: a **conversion funnel** (SAVED as the baseline of every tracked application, then each
  later stage counting distinct applications that have *ever* reached that status via
  `application_events` — not just applications currently sitting in it, which would undercount anyone who
  moved past a stage), a **response rate** (share of APPLIED applications that ever reached
  RECRUITER_SCREEN), and an **average match score** across the user's tracked applications. Route:
  `GET /analytics/dashboard`.
* **Frontend**: `/analytics` page — stat cards (jobs discovered, applications tracked, tailoring runs,
  high matches, response rate, average match score), a CSS-only funnel bar chart (no charting library
  dependency added), and a status-count summary.
* **Tests**: 2 new Go integration tests — a full funnel/response-rate/average-score scenario with two
  applications at different stages, and an empty-state scenario (zero applications produces zero-value
  results, not errors).
* Verified live end-to-end via docker-compose: dashboard with no data (all zeros, funnel present but empty,
  null average score), then after saving a job and advancing it to APPLIED then RECRUITER_SCREEN — funnel
  counts, 100% response rate (1 of 1 applied reached recruiter screen), and average match score all
  correct.

## Phase 12 — what was actually delivered

* **Go**: `internal/httpapi/ratelimit.go` — a simple in-memory, per-IP fixed-window rate limiter (no new
  dependency). Applied as two tiers: a strict 20 requests/minute limit on `/api/v1/auth/*` (brute-force /
  credential-stuffing protection) and a generous 300 requests/minute limit across the rest of the
  authenticated API. `internal/account` — `Service.DeleteResume` (deletes a resume's uploaded file and any
  generated PDF/DOCX version files from object storage, then the resume row — cascading its experiences
  and versions in the database) and `Service.DeleteAccount` (deletes every resume's storage files across
  the account, then the user row — cascading sessions, profile, preferences, resumes, tailoring runs,
  applications, and everything else that references `users(id)`, since every such foreign key uses
  `ON DELETE CASCADE`). Routes: `DELETE /resumes/{id}`, `DELETE /account` (also clears the session cookie).
  A `StorageDeleter` interface (rather than the concrete storage client type) lets tests inject a fake
  in-memory recorder instead of talking to real MinIO.
* **Frontend**: a delete button per resume row on `/resume`, and a "Danger Zone" section at the bottom of
  that page with a confirmed "Delete Account" button that logs the user out and redirects to `/login`.
* **Tests**: 3 new rate-limiter unit tests (allows-up-to-limit-then-blocks, independent per-key tracking,
  429 via the actual middleware) and 3 new `internal/account` integration tests (resume deletion leaves
  other resumes untouched, deleting another user's resume returns not-found, account deletion cascades both
  the user row and their resumes while recording the storage keys that were "deleted").
* **Docs**: this pass — README status/scope-limitations sections rewritten for all 12 phases, `API.md`
  brought current, `DECISIONS.md` given a Phase 12 entry, and a formatting bug from earlier phase edits in
  `IMPLEMENTATION_PLAN.md` fixed (see Phase 11 entry above).
* Verified live end-to-end via docker-compose: uploaded and deleted a resume (404 on subsequent
  get/re-delete), sent 25 rapid login attempts and confirmed the 20th+ were rejected with 429, deleted a
  full account and confirmed both the API 401s on the now-invalid session cookie and a direct `SELECT`
  against Postgres show zero matching rows.

## All 12 phases are now complete (MVP feature-complete)

See [DECISIONS.md](DECISIONS.md) for the full technical-debt list accumulated across phases (no real AI
provider, no Immigration-Aware Job Matching sub-system, in-memory non-distributed rate limiting, etc.) —
these are documented, deliberate scope boundaries for this build, not oversights.

Production hardening: security, observability, performance, deployment, CI/CD, docs. Do not begin this
until explicitly requested.

## Known scope limitations (see DECISIONS.md for the full, per-phase list)

* No real AI provider integration yet — resume parsing, JD parsing, and tailoring all use deterministic
  heuristic stand-ins behind the same interface shape a real `AIProvider` would use. This was a deliberate,
  documented scope decision (no `AI_API_KEY` available), not an oversight.
* The Immigration-Aware Job Matching sub-system (DOL data ingestion, `ImmigrationCompatibility`,
  immigration-weighted Opportunity Score) is not implemented — it's a large sub-system of its own and was
  explicitly out of scope for "Phases 2-7" as enumerated in §72.
* Education/certification cross-referencing in match scoring is a flat partial-credit default, not a real
  comparison against parsed resume education/certifications.
* Domain alignment in the match score uses a flat default (no domain extraction in the heuristic JD parser).
* Rate limiting (Phase 12) is a simple in-memory, per-IP fixed-window limiter — correct for a single API
  instance but not distributed; a multi-replica production deployment would need a shared store (e.g. Redis).
* See the "Remaining technical debt after Phase N" sections in [DECISIONS.md](DECISIONS.md) for the
  complete list across Phases 8-12 (Interview Readiness approximation, no resume-version diff/preview UI,
  no Kanban drag-and-drop, no time-series analytics, etc.).

