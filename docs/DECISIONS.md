# ApplyForge — Decisions Log

Chronological record of concrete architectural decisions actually made while building the repo (as opposed
to the aspirational design in MASTER_REQUIREMENTS.md). Update this file whenever a phase changes or refines
a prior decision, and state why.

## Phase 0

1. **Monorepo, not polyrepo.** `apps/web`, `apps/api`, `apps/ai-worker` in one GitHub repo for atomic
   cross-service changes during early development.
2. **Go API framework:** `chi` router + stdlib `net/http`, `slog` for structured JSON logs, module path
   `github.com/lalithlochan/applyforge/apps/api`. No web framework beyond chi — keeps the dependency surface
   minimal per §68/§76.
3. **Go DB access deferred:** `pgx` pool wiring is included for `/ready` (DB ping), but `sqlc`/`goose` are
   not wired until Phase 1 introduces the first real tables — no schema exists yet, so generating code
   against it would be premature.
4. **Python AI worker framework:** FastAPI + Pydantic v2, `uvicorn` for the dev server, `ruff` for lint,
   `pytest` for tests. Package layout follows §70 (`app/api`, `app/models`, `app/providers`, etc.) but only
   `app/main.py` and a health router exist in Phase 0 — the rest are empty-with-`.gitkeep` placeholders so
   the intended structure is visible without inventing code that has no behavior yet.
5. **Frontend:** Next.js (App Router, TypeScript, Tailwind, ESLint) via `create-next-app`, package manager
   `pnpm` per §56. shadcn/ui, TanStack Query, Zod, React Hook Form, Recharts are documented as required
   dependencies but not installed in Phase 0 since there are no components/forms yet needing them.
6. **Local infra:** `docker-compose.yml` provisions `postgres:16` with a healthcheck, plus buildable
   `api` and `ai-worker` services. `web` intentionally runs outside Docker (`pnpm dev`) for fast HMR, per
   common Next.js local-dev practice.
7. **CI:** a single GitHub Actions workflow (`.github/workflows/ci.yml`) with three independent jobs
   (`web`, `api`, `ai-worker`), each doing lint + build/test for its own service, gated on path filters so
   unrelated changes don't run irrelevant jobs (added once each app's toolchain files exist).
8. **No secrets in the repo.** `.env.example` documents every variable from §60; `.gitignore` excludes
   `.env*` (except `.env.example`).
9. **Go toolchain version:** `go.mod` declares `go 1.24`, but `go mod tidy` bumped the effective minimum to
   `1.25.0` because `pgx/v5` v5.10.0 requires it. The Docker build image and CI use `1.25`+ (locally verified
   against Go 1.26.2) to satisfy this — still within the "Go 1.24+" requirement in §4.
10. **Local Postgres port:** docker-compose maps the container's `5432` to host port **5433** (not 5432)
    because another local Docker Postgres instance already occupied 5432 on this machine. Container-to-
    container traffic (api → postgres) is unaffected since it uses the internal Docker network on 5432;
    only host-side access (e.g. `psql` from the Mac, or running the Go API outside Docker) needs port 5433.
    `.env.example` reflects this.
11. **Python interpreter:** system default was `python3.9`; used Homebrew `python3.13` to create
    `apps/ai-worker/.venv`, satisfying the "Python 3.12+" requirement in §4.

## Not yet decided (deferred to later phases)

* AIProvider concrete vendor — Phase 4.
* Object storage client library choice (Go + Python) — Phase 2.

## Phase 1

1. **Auth ownership:** the Go API owns authentication directly rather than delegating to a managed provider
   or NextAuth in the frontend — keeps authorization centralized in one place per §9 ("every user-owned
   resource must require authorization") and avoids adding a paid dependency before there are users.
   `internal/auth` is isolated enough to swap later if needed.
2. **Session strategy:** opaque random tokens (not JWTs) in an `HttpOnly` cookie, backed by a `sessions`
   table storing only a SHA-256 hash of the token. Chosen over JWTs so sessions can be revoked server-side
   (logout, future "log out all devices") without needing a blocklist. Default TTL 30 days
   (`auth.SessionTTL`).
3. **Google OAuth:** implemented directly with `golang.org/x/oauth2` (authorization-code flow) plus the
   `openidconnect.googleapis.com/v1/userinfo` endpoint, rather than a heavier auth library — matches
   "don't create abstraction for abstraction's sake" (§68). Missing `GOOGLE_CLIENT_ID`/`SECRET`/`REDIRECT_URL`
   causes the Google endpoints to return `503` instead of crashing the server, so local dev works without
   Google credentials configured.
4. **job_preferences includes immigration fields now.** The master spec's Immigration-Aware Job Matching
   section says to "extend JobPreferences" with granular fields (`requires_h1b_transfer` vs
   `requires_new_h1b_cap_sponsorship`, green-card/PERM preferences, confidence threshold). Added at table
   creation time (Phase 1) rather than as a later migration, since `job_preferences` didn't exist before this
   phase — avoids schema churn and keeps the two concepts distinct from day one, per the spec's explicit
   warning never to collapse them into one boolean.
5. **sqlc + goose tooling:** installed via `go install` (not present as system packages);
   `apps/api/sqlc.yaml` generates into `internal/database/gen` (package `db`, `pgx/v5` driver). Repositories
   wrap generated `*db.Queries` and convert `pgtype.*` values to plain Go types (`*string`, `*int32`,
   `*time.Time`, `uuid.UUID`) at the package boundary so domain code never touches pgx types directly.
6. **Repository/service testability:** `auth.Service` depends on small `UserStore`/`SessionStore` interfaces
   (not concrete repository types), enabling fast unit tests with in-memory fakes
   (`internal/auth/service_test.go`) alongside real-Postgres integration tests. Integration tests
   (`*_integration_test.go` in `users`, `profile`, `preferences`, `auth`) use `internal/testdb.OpenTx`, which
   runs each test inside a transaction that's rolled back afterward — no manual fixture cleanup, and tests
   skip automatically (not fail) when `DATABASE_URL` is unset.
7. **CI now runs a real Postgres service** for the `api` job (installs `goose`, runs migrations, then
   `go test ./...` with `DATABASE_URL` set) so repository integration tests actually execute in CI instead of
   always skipping.
8. **API response shape:** `PATCH /profile` and `PATCH /preferences` are full upserts (whole-object
   replace), not partial merges — simplest correct semantics given the underlying SQL is `INSERT ... ON
   CONFLICT DO UPDATE SET <every column> = EXCLUDED.<column>`. The frontend always sends complete form state
   to avoid accidentally nulling out fields; this is called out in API.md so future frontend work doesn't
   assume PATCH semantics are partial.
9. **No OpenAPI generation yet.** Deferred until the endpoint surface is large enough to justify the
   tooling/maintenance cost — tracked as technical debt, revisit by Phase 6 at the latest.
10. **Frontend auth/onboarding:** a fetch-based `lib/api.ts` client (credentials included on every request),
    zod schemas with `z.input`/`z.output` split for fields that need comma-separated-string → string[]
    transforms (required for `react-hook-form`'s `useForm<Input, Context, Output>` generic to type-check),
    a single onboarding page with two in-memory steps (not separate routes) so step-1 data can be resubmitted
    together with step-2 data — avoids the upsert-is-full-replace footgun in decision 8 above.
11. **shadcn/ui still not installed.** Forms use plain Tailwind-styled inputs; introducing the shadcn CLI
    (interactive, pulls in a component registry) is deferred until Phase 6 when the jobs UI needs a larger,
    reusable component set.

### Remaining technical debt after Phase 1

* No rate limiting, CSRF token (beyond OAuth `state`), account/resume deletion, or email verification
  enforcement yet — appropriate for Phase 12 unless a concrete need arises sooner.
* No OpenAPI spec generated yet.
* Onboarding UI has no client-side route guard preventing an unauthenticated user from viewing
  `/onboarding` (only `/dashboard` checks the session); low risk since the API itself enforces
  authorization, but worth tightening once more authenticated pages exist.

## Phases 2-7 (built in a single pass at explicit user request)

The user asked to "build phase 2-7" in one continuous session, overriding the normal one-phase-at-a-time
cadence. Given the enormous combined scope, the following deliberate scope decisions were made up front and
held throughout, rather than discovered ad hoc partway through:

1. **No real AI provider integration.** No `AI_API_KEY` was available. Instead of blocking or faking a
   provider response, every AI-worker endpoint that MASTER_REQUIREMENTS.md describes as AI-driven (resume
   parsing, JD requirement extraction, tailoring suggestion generation) was implemented as a **deterministic,
   regex/keyword-based heuristic** behind the exact same request/response contract a real LLM call would
   use (`app/resume/parsing.py`, `app/jobs/parsing.py`, `app/tailoring/heuristics.py`). This keeps the full
   pipeline (upload → parse → match → tailor → approve) genuinely functional and testable end-to-end without
   external network dependencies or API costs, and confines the future "plug in a real model" change to
   those three files plus a shared skill dictionary (`app/core/skills_dictionary.py`) — the Go side,
   database schema, and API contracts don't change when a real provider is added.
2. **Immigration-Aware Job Matching sub-system is out of scope.** MASTER_REQUIREMENTS.md includes a large,
   separate section on H-1B/green-card/PERM compatibility scoring backed by DOL data ingestion. This is a
   substantial sub-system in its own right (its own tables, evidence model, company-alias resolution, data
   refresh pipeline) and isn't enumerated under "Phase 2-7" in §72's phase list, so it was explicitly not
   built. `job_preferences` already has the immigration preference *fields* (Phase 1), but no
   `ImmigrationCompatibility`/DOL ingestion exists.
3. **Background worker runs in-process, not as a separate `cmd/worker` binary.** §69 suggests
   `cmd/api`/`cmd/worker` as separate binaries; given the time budget, the worker (`internal/background`)
   runs as a goroutine inside the single `cmd/api` process. This is a smaller, still-correct MVP shape (one
   fewer container/Dockerfile/compose service to maintain) and is easy to split into a separate binary later
   if worker load ever needs independent scaling — nothing about the `background.Worker`/`Queue` API design
   assumes in-process execution.
4. **MinIO for local S3-compatible storage** (`internal/storage`, `minio-go` client), auto-creating the
   bucket on startup. Chosen over the full AWS SDK v2 for a much smaller dependency surface; the same client
   works against Cloudflare R2/AWS S3 in production since both speak the S3 API.
5. **Real public job board connectors, not mocked ones.** Verified live network access works in this
   environment and confirmed real, unauthenticated public APIs for all three required sources (Greenhouse:
   `robinhood`, Lever: `lever`, Ashby: `ramp`), then built `internal/jobs`'s connectors against the real
   APIs and validated live ingestion (128 Greenhouse + 142 Ashby jobs on first sync). Connector *tests* still
   use `httptest` fixtures (no live network in CI), per §54.
6. **Deterministic scoring is genuinely pure and DB-free.** `internal/matching`'s `Score(Input) Result` and
   `internal/tailoring`'s `ComputeAlignment` take plain Go structs/maps, not repository types — this made the
   golden tests (§55) fast, dependency-free unit tests rather than integration tests, and is what let the
   transferable-skill partial-credit design get fixed quickly once the first test run caught a threshold bug
   (see below).
7. **Transferable-skill credit is proportional, not a binary cutoff.** The first implementation only counted
   a transfer as "meaningful" above a fixed score threshold (60), which caused the seeded Kafka→SQS (55,
   MEDIUM) pair to be silently treated as "missing" instead of "transferable" — failing the golden test that
   the spec explicitly asks for. Fixed by making skill-coverage credit proportional to
   `transferability_score/100`, capped at 0.8 so a transfer can never equal direct-skill credit. All
   transfers (even LOW) now appear in the `transferable_skills` output, distinct from `missing_required_skills`.
8. **Company display names come from `job_sources`, not connector responses.** Greenhouse/Lever/Ashby board
   APIs are scoped to a single company and don't return a company display name field — the first ingestion
   implementation mistakenly used the lowercase board token as the company name (`UpsertCompany` then
   overwrote the properly-cased seeded name). Fixed by joining `companies` into `ListJobSources` and passing
   the known company id/name into `Ingest` explicitly, rather than trying to re-derive it per job.
9. **Resume Alignment Score is a separate, simpler formula from Job Match Score.** `ComputeAlignment` in
   `internal/tailoring` duplicates a small amount of coverage-ratio logic from `internal/matching` rather
   than importing/reusing it, since the two scores answer different questions (§20 vs §23) and are computed
   at different times (immediately, without eligibility/seniority/location factors). Documented duplication,
   not accidental.
10. **Jobs list doesn't include match scores; each `JobCard` fetches its own score client-side.** Keeps
    `GET /jobs` fast and cacheable independent of the requesting user; per-card TanStack Query calls to
    `GET /jobs/{id}/match` are cheap since scoring is deterministic Go, not an AI call — this is explicitly
    allowed as "AI explanations: lazy" only applies to genuinely AI-backed operations.
11. **Tooling quirk discovered and documented in `/memories/debugging.md`:** the file-creation/editing tools
    occasionally produced duplicate `package X` declaration lines in newly created Go files, and in one case
    `read_file` showed content that didn't match what was actually on disk (a real function definition that
    a prior edit had failed to persist). Both were caught via `grep`/`go vet` discrepancies and fixed by
    writing directly to the file via a terminal script, not by re-attempting the same tool call.

### Remaining technical debt after Phases 2-7

* No real AI provider — see decision 1 above. Swapping one in requires: adding an `AIProvider` interface in
  `app/providers/`, implementing it against a real model, and switching the three heuristic call sites to
  use it (with response validation, since heuristic functions currently can't "fail" the way an LLM call can).
* Domain alignment (10% of Job Match Score) and education/certification alignment (5%) use flat default
  partial credit — the heuristic JD parser doesn't extract domains, and match scoring doesn't cross-reference
  a candidate's parsed resume education/certifications yet.
* No `ai_usage` cost/latency logging table — nothing to log yet without a real provider.
* Job listing pagination exists (`limit`/`offset`) but there's no "load more"/infinite-scroll UI yet — the
  frontend only shows the first page.
* Tailoring `MAX_MATCH` mode can still only draw from the skill dictionary already known to the heuristic
  parser; it doesn't invent genuinely novel JD terminology the way a real LLM might.
* Save/bookmark job, applications tracking, analytics, Quick Prep, Defend This Bullet, and PDF/DOCX
  generation are unbuilt (Phases 8-11).

## Phase 8 — Quick Prep, Defend This Bullet, Make Me Qualified, Interview Readiness, learning plans

1. **Quick Prep cache is generic/shared across users; transferable-skill personalization is never cached.**
   `quick_prep_modules` is keyed only by `normalized_skill`, with no user column, since the underlying
   content (what a skill is, why it matters, common interview questions) is the same for every user. The
   "what you already know that transfers" list, however, is inherently per-candidate — so it's computed at
   request time from the caller's own candidate skills via the matching engine's `transferable_skills`
   lookup, merged onto the cached generic module, and never persisted back into the cache.
2. **Defend This Bullet takes bullet text + skills directly in the request body, not a suggestion/experience
   ID.** There's no existing "get one tailoring suggestion by ID alone" repository method, and the caller
   (a tailoring suggestion card or a resume experience row) already has both the bullet text and its
   associated skills on hand client-side. Resolving an ID server-side would have required adding new
   plumbing across the `tailoring`/`resume` packages for no functional benefit.
3. **Interview Readiness is derived from the existing match `ComponentScores`, not a new dedicated signal.**
   §35 defines six weighted components (Core Language 20, Backend Fundamentals 20, Required Technology 25,
   System Design/Domain 15, Experience Examples 10, Question Preparedness 10). There's no tracked signal yet
   for some of these (e.g. "question preparedness" would ideally reflect actual Quick Prep/Defend Bullet
   usage, which isn't tracked). The current implementation approximates each component from the closest
   existing match component (e.g. Required Technology from `MustHaveSkillCoverage`) and documents this
   explicitly in code comments as product guidance, not a scientific assessment — matching the spec's own
   framing of the score.
4. **`nil` Go slices must never be marshaled as JSON for AI-worker requests.** A Go slice that is `nil`
   marshals to JSON `null`. Pydantic's `Field(default_factory=list)` only supplies a default when a field is
   *omitted* from the request body — an explicit `null` still fails validation against `list[str]`. This
   caused quick-prep to fail with a 422 on first live test (`transferable_from: null`). Fixed by normalizing
   `nil` to `[]string{}` before marshaling in all three `internal/aiclient` learning methods
   (`GenerateQuickPrep`, `DefendBullet`, `GenerateLearningPlan`). Worth checking for the same pattern if any
   future `aiclient` method takes an optional slice parameter.
5. **Migrations for Phase 9-10 tables were created ahead of their implementation.** `resume_versions`,
   `applications`, `application_events`, and `application_answers` were designed and migrated in this pass
   since the schema work was quick to batch alongside Phase 8's tables, but no Go/Python code reads or
   writes them yet — they're inert until Phase 9/10 are implemented.

### Remaining technical debt after Phase 8

* Interview Readiness component mapping (decision 3 above) is an approximation; a more accurate version
  would need dedicated signals (e.g. actual language/framework matched-skill breakdown, tracked Quick
  Prep/Defend Bullet engagement) that don't exist yet.
* Quick Prep's curated `CONTENT_BANK` covers 8 technologies; everything else falls back to a generic,
  clearly-labeled-as-generic module. Expanding coverage is pure content work, not an architecture change.
* No rate limiting or per-user usage caps on Quick Prep/Defend Bullet/Learning Plan generation yet — these
  are lightweight heuristic calls (no real AI cost), but that will need revisiting if a real AI provider is
  ever plugged in.

## Phase 9 — Resume generation (PDF, DOCX, versioning, preview)

1. **Reused the existing `ResumeProfile` model for document generation instead of a new schema.** The
   PDF/DOCX generator endpoints (`POST /v1/documents/pdf`, `POST /v1/documents/docx`) take the exact same
   `ResumeProfile` Pydantic model already used for resume parsing (`app/resume/models.py`). This avoids a
   parallel "document content" schema that would need to be kept in sync, and means the Go side's
   `mergeContent` function only has to produce one shape (`aiclient.ResumeProfile`), not a second
   document-specific one.
2. **The merge step reads the base resume's `parsed_profile` directly, not the `resume_experiences` table.**
   Both are populated from the same parse operation with identical shape (see `resume/worker.go`), so
   re-deriving experiences from the separate table would be redundant. `mergeContent` unmarshals
   `parsed_profile` straight into `aiclient.ResumeProfile` and merges suggestions onto it in memory.
3. **Experience-section suggestions are matched back to their bullet by an exact `original_text` string
   match**, not a stored bullet ID/index. The tailoring heuristics already set `original_text` to the exact
   bullet text sent to the AI worker (`app/tailoring/heuristics.py`), so this is a reliable match without
   adding a new foreign key from `tailoring_suggestions` to `resume_experiences`.
4. **Downloads are proxied through the Go API, not served via presigned MinIO URLs.** `GET
   /resume-versions/{id}/download?format=pdf|docx` fetches the bytes from storage server-side and streams
   them with the right `Content-Type`/`Content-Disposition`, keeping the storage bucket itself private and
   avoiding the extra complexity of presigned-URL expiry handling for what is a low-traffic download path.
5. **`resume.NewRepositoryFromQueries` was added** (previously missing, unlike `users`/`jobs`/
   `jobrequirements`) so `internal/resumeversion`'s integration tests could create a fixture resume row
   without depending on a full `*database.Pool`.

### Remaining technical debt after Phase 9

* PDF/DOCX rendering is a single fixed template (no user-selectable resume design/theme, no page-count
  awareness beyond fpdf2's automatic page breaks).
* No resume version diff/preview UI yet in the frontend beyond the download links — a future pass could
  render an inline HTML preview before download.
* `mergeContent`'s bullet matching is an exact string match; if a user edits a bullet's text in their
  master resume after a tailoring run was created but before generating a version, the match would silently
  fail to apply that suggestion (falls back to leaving the original bullet in place, not an error).

## Phase 10 — Applications tracking (Kanban, table, application answers, events)

1. **"Save Job" is an idempotent upsert, not a plain insert.** `CreateApplication` (written in the Phase 8
   pass) is `INSERT ... ON CONFLICT (user_id, job_id) DO UPDATE`, so calling Save again for the same job
   (e.g. after generating a new resume version) just re-attaches the resume version without resetting an
   in-progress application back to SAVED or duplicating the row.
2. **Status transitions are click-to-advance, not full drag-and-drop, in the Kanban view.** A real DnD
   library is a meaningfully sized new frontend dependency for a single-user MVP; each card has a "Move to
   next stage" button (and the table view has a plain status dropdown) that calls the same
   `PATCH /applications/{id}` endpoint. Revisit if multi-column reordering ends up mattering in practice.
3. **Every status change is logged as an `application_events` row, but no-op transitions are not.**
   `Service.ChangeStatus` compares the current and requested status before logging, so re-saving the same
   status (e.g. from a stale frontend state) doesn't pollute the history with duplicate identical events.
4. **`GET /application-answers` returns zero-value defaults for a user with no saved answers, not a 404**,
   matching the same convention already established for `/profile` and `/preferences` — the frontend can
   render the answers form immediately without a loading branch for "not found yet".

### Remaining technical debt after Phase 10

* No bulk "apply with saved answers" flow yet — application answers are stored but nothing auto-fills an
  external ATS form with them (that would require per-ATS integration, out of scope here).
* Kanban view has no drag-and-drop; status changes are single-step "move to next" or a dropdown, not
  free-form column-to-column dragging.
* No application-level attachments (cover letter, etc.) beyond the linked `resume_version_id`.

## Phase 11 — Analytics (conversion funnel, response rates, match-score analytics)

1. **The conversion funnel counts applications that ever reached a stage, not applications currently
   sitting in it.** Built from `application_events` (`CountApplicationEventsByToStatusForUser`), so an
   application that's now at OFFER still counts toward the APPLIED and RECRUITER_SCREEN funnel stages it
   passed through earlier. Using current-status snapshots instead would make the funnel look artificially
   narrow at every stage past the first.
2. **SAVED is the funnel's baseline, computed from total tracked applications, not an event.** `Save`
   (via `CreateApplication`) doesn't log an `application_events` row for the initial creation — only
   `Service.ChangeStatus` logs events, for actual transitions — so the SAVED stage count comes from
   `CountApplicationsByStatusForUser`'s total rather than an event count.
3. **Average match score is computed in Go from `applications.match_score`, not a new SQL `AVG()` query.**
   The existing `ListForUser` repository call was already available and the dataset size (one user's
   tracked applications) makes an in-memory average simpler than adding another sqlc query.
4. **No charting library dependency added for the funnel visualization.** The `/analytics` page renders
   the funnel as CSS-only proportional bars (each stage's count relative to the largest stage), consistent
   with the app's general preference for minimal new frontend dependencies.

### Remaining technical debt after Phase 11

* Analytics are user-scoped only; there's no cross-user/admin view (not needed for a single-user-per-account
  product at this stage).
* No time-series/trend view (e.g. applications per week) — the dashboard is a current-snapshot view only.
* `JobsDiscovered` counts all ACTIVE jobs in the system, not jobs discovered specifically for this user's
  preferences/matches — a coarse global signal rather than a personalized one.

## Phase 12 — Production hardening (rate limiting, account/resume deletion)

1. **Rate limiting is a simple in-memory, per-IP fixed-window limiter — no new dependency.** Two tiers: a
   strict 20 req/min limit on `/api/v1/auth/*` (brute-force/credential-stuffing protection) and a generous
   300 req/min limit across the rest of the authenticated API. This is correct for a single API instance
   (the deployment topology documented in DEPLOYMENT.md) but isn't distributed — a multi-replica deployment
   behind a load balancer would need a shared store (e.g. Redis) so limits are enforced consistently across
   instances, not per-instance.
2. **Account/resume deletion only needs to clean up object storage manually — the database cascades
   everything else.** Every foreign key referencing `users(id)` or `resumes(id)` across the entire schema
   uses `ON DELETE CASCADE` (verified by grepping every migration), so a single `DELETE FROM users WHERE
   id = $1` removes sessions, profile, preferences, resumes, resume_experiences, resume_versions,
   candidate_skills, tailoring_runs, tailoring_suggestions, job_matches, applications, application_events,
   application_answers, and learning_plans in one statement. `internal/account` exists specifically to
   delete the object-storage files (uploaded resumes, generated PDF/DOCX versions) that a DB cascade can't
   reach, in the correct order (storage first, then the DB row) so a mid-failure never leaves an orphaned
   storage file with no DB record pointing to it — the reverse order could leave a dangling reference.
3. **`internal/account` depends on a `StorageDeleter` interface, not the concrete `*storage.Client` type.**
   This is the only package in the codebase that needed to unit-test storage-deletion behavior without a
   real MinIO connection, so a minimal interface (just the `Delete` method) was introduced there rather than
   changing `storage.Client`'s public shape everywhere else.
4. **`DELETE /resumes/{id}` and `DELETE /account` are mounted from a new `internal/account` package, not
   from `internal/resume`.** Resume deletion needs to know about `resumeversion` (to find generated PDF/DOCX
   storage keys), and `resumeversion` already depends on `resume` — so putting deletion logic inside
   `resume` would create an import cycle. A small coordinating package one level up avoids that.

### Remaining technical debt after Phase 12

* No structured audit log of account/resume deletions (who deleted what, when) beyond normal request logs.
* No "export my data" endpoint alongside deletion (GDPR-style data portability) — out of scope for this pass.
* No CI/CD pipeline changes in this pass beyond what already existed from Phase 0 (`.github/workflows/ci.yml`
  already runs lint/test/build for all three services; Phase 12 didn't add deployment automation).

## Post-Phase-12 — Real OpenAI integration (supersedes Phases 2-7 decision 1)

Once a real `OPENAI_API_KEY` became available locally, the "no real AI provider" scope decision from
Phases 2-7 was revisited and implemented, exactly along the "future plug-in" path that decision anticipated.

1. **`app/providers/openai_provider.py`** is the single integration point: `is_configured()` checks
   `OPENAI_API_KEY` presence, and `structured_completion(...)` wraps `client.chat.completions.parse(...,
   response_format=<PydanticModel>)` (OpenAI structured outputs), returning a validated Pydantic instance
   directly — no manual JSON-schema authoring or manual `json.loads`/validation. Model is configurable via
   `OPENAI_MODEL` (default `gpt-4o-mini`).
2. **Each of the three heuristic modules got an `_ai` sibling function**, not a replacement:
   `parse_resume_text_ai`, `parse_job_requirements_ai`, `generate_tailoring_ai` in the same three files
   named in the original decision. The heuristic functions are untouched and still fully tested.
3. **Route handlers try AI first, then fall back to the heuristic** — `if is_configured(): try: <ai
   function> except AIProviderError: logger.warning(...)` then always falls through to the original
   heuristic call. This means the app degrades gracefully (never 500s) if the key is unset, revoked,
   rate-limited, or OpenAI has an outage, and every environment without a key behaves exactly as it did
   before this change.
4. **`TailoringSuggestion.section`, `.source`, `.risk_level` are typed as `Literal[...]` (not plain
   `str`).** This was necessary, not cosmetic: a live test call showed the model returning
   `risk_level: "low"` (lowercase) while the Go API's `tailoring_suggestions` table has a hard
   `CHECK (risk_level IN ('LOW','MEDIUM','HIGH'))` constraint — a lowercase value would have caused a
   500 on `POST` once persisted. Using `Literal` types makes OpenAI's structured-output constrained
   decoding itself only emit the exact allowed tokens (schema-level guarantee, not a post-hoc string
   check), and a defensive `field_validator` on `risk_level` additionally normalizes casing before the
   `Literal` check as a second layer, in case a future model/version drifts.
5. **API key is never hardcoded anywhere.** `docker-compose.yml`'s `ai-worker` service reads
   `OPENAI_API_KEY: "${OPENAI_API_KEY:-}"` / `OPENAI_MODEL: "${OPENAI_MODEL:-gpt-4o-mini}"` — sourced from
   the shell environment (or an untracked `.env` file) at `docker compose up` time. `.env.example` documents
   both variables with empty placeholder values alongside the older `AI_PROVIDER`/`AI_MODEL`/`AI_API_KEY`
   placeholders (kept for reference, not currently read by any code path).
6. **Verified live (not just mocked in tests):** with the real key wired into the running container, both
   `/v1/tailoring/suggest` and `/v1/jobs/parse-requirements` returned genuinely AI-generated (non-templated)
   output with zero fallback-warning log lines, confirming the AI path — not the heuristic fallback — is
   what actually ran.

### Remaining technical debt after this change

* No `ai_usage` cost/latency/token logging yet (flagged as unbuilt back in Phase 2-7 remaining debt, still
  true) — every real OpenAI call now has a genuine cost, so this is more important to add than before.
* No retry/backoff on transient OpenAI errors (rate limits, timeouts) — a single failure falls straight
  through to the heuristic for that one request rather than retrying.
* No per-user or global rate limiting specifically on AI-backed endpoints (existing general API rate
  limiting from Phase 11/12 still applies, but doesn't distinguish AI-cost-bearing requests).
* `app/learning/` (Quick Prep, Defend Bullet, Learning Plan) still uses only heuristics/content-bank lookups
  — deliberately out of scope for this pass, a natural follow-up now that the integration pattern exists.
* Resume-parsing data-quality issues noted during manual QA (occasional garbled job titles, e.g. a date
  fragment or the literal placeholder "Experience") and occasional truncated tailoring bullets were
  observed with the heuristic path before this change; not yet re-verified against the AI path in
  production usage.

## Job source coverage: single-company connectors vs. aggregators

1. **Problem:** Greenhouse/Lever/Ashby connectors each require a specific company's `board_token` known in
   advance — none of them expose a "search every company" endpoint. Company coverage was therefore capped
   at whatever we individually seeded (grew 3 → 13 → 36 companies across migrations `00013`/`00026`/`00027`,
   all live-verified token-by-token). A curated seed list can never represent "the whole job market."
2. **Decision:** Add a second *kind* of connector — a multi-company aggregator — rather than only continuing
   to seed more single-company tokens. Chose **Arbeitnow** (`https://www.arbeitnow.com/api/job-board-api`):
   free, no API key/signup required, and a single feed genuinely returns postings from hundreds of distinct
   real companies (confirmed live), unlike Adzuna/USAJobs which need user-provided API keys.
3. **`RawJob.CompanyName` repurposed:** the field already existed on `RawJob` but was previously unused by
   `Ingest()` (Greenhouse/Lever/Ashby don't return a real company display name, so it was ignored in favor
   of the job_sources-configured company). `Ingest()` now checks `raw.CompanyName`: if non-empty, it
   upserts/reuses a company row per job (cached per poll to avoid redundant upserts for repeat companies),
   overriding the source's configured company; if empty, behavior for existing single-company sources is
   unchanged.
4. **`job_sources.company_id` stays `NOT NULL`:** rather than relaxing the schema, a single placeholder
   "Arbeitnow Aggregator" company row satisfies the FK for the one `ARBEITNOW` job_sources config row
   (`board_token='global'`, migration `00028`) — the *real* per-job companies are resolved dynamically in
   `Ingest()`, not from this placeholder.
5. **Pagination safety cap:** Arbeitnow's own docs say the feed refreshes hourly and ask API consumers not
   to abuse it. `ArbeitnowSource.MaxPages` (default 5, ~250 jobs/page) caps how much of the feed one poll
   fetches, following `links.next` until either the cap or a `null` next link is reached.
6. **Verified live:** rebuilt the API container, triggered a real sync — distinct company count jumped from
   36 to **401**, 784 new Arbeitnow jobs inserted, and a second sync was fully idempotent (0 inserted, 800
   updated), confirming upsert/dedup correctness holds for the new dynamic-company path.
7. **Remaining technical debt:** no `page`-level cursor persistence between polls (each poll restarts from
   page 1, relying on `MaxPages` + upsert idempotency rather than resuming where the last poll left off);
   no separate "aggregator vs. single-company" flag on `JobSourceConfig` (the branch is driven purely by
   whether `raw.CompanyName` is populated, which is simple but implicit — worth making explicit if a second
   aggregator connector is added later).

## Big architecture shift: ingest -> enrich -> filter -> retrieve -> rank -> async tailor

Target end-state (12-phase roadmap, tracked in repo memory
`applyforge-architecture-roadmap.md`): continuously ingest a large job universe, normalize/dedupe,
AI-enrich each canonical job once, aggressively hard-filter, semantically retrieve, AI-rerank the
finalists, then asynchronously build a tailored resume. This section covers Phase A, the first
concrete step.

### Phase A: async ingestion via the existing background queue

1. **Reused `internal/background`, didn't build a new queue.** A Postgres-backed queue
   (`background_jobs`, `FOR UPDATE SKIP LOCKED`, retry/backoff, dead-letter) already existed for
   resume parsing. `IngestionService` now depends on it too: `EnqueueSyncTasks()` enqueues one
   `sync_job_source` task per enabled source instead of `SyncAll()` polling every source
   sequentially in one request/goroutine. Explicitly **not** introducing Kafka/SQS at this scale —
   Postgres `SKIP LOCKED` comfortably handles tens of concurrent workers.
2. **Multiple worker goroutines, not one.** `main.go` now spins up `BACKGROUND_WORKER_COUNT`
   (default 5) worker goroutines sharing the same queue/handlers. Verified live via the
   `background_jobs.locked_by` column showing distinct worker names claiming rows concurrently.
3. **The admin sync endpoint and scheduler are now non-blocking.** `POST /admin/job-sources/sync`
   returns `202 Accepted` immediately (enqueues, doesn't wait for providers to respond); the
   scheduler enqueues on its interval instead of running a synchronous sweep.

### Phase D (partial): eager AI enrichment, scoped to new jobs only

4. **JD parsing moves from purely-lazy to eager-on-insert.** `Ingest()` enqueues an `enrich_job`
   task for every newly inserted job, processed by a new `EnrichWorker` that calls the existing
   `jobrequirements.Service.GetOrParse` (already cached by `content_hash` — this part pre-existed
   and needed no change). Falls back to lazy on-demand parsing (unchanged) for anything not eagerly
   enriched yet.
5. **Incident: scoped eager enrichment to inserts only, not updates, after a real cost scare.** The
   first implementation enqueued `enrich_job` for every upserted job, inserted *or* re-seen/updated.
   Since ~6,252 pre-existing jobs had never been parsed (enrichment was previously lazy-only), the
   very first sync cycle after deploying this enqueued ~6,982 enrich tasks — the overwhelming
   majority being genuine, billable OpenAI calls, not cache hits. Caught this at 324 real calls
   (cross-checked `job_requirements` row count against `ai-worker` request logs), stopped the `api`
   container, deleted the unprocessed backlog rows from `background_jobs`, and restricted the
   trigger to `upserted.Inserted == true` only. Verified live that a subsequent sync no longer grows
   `job_requirements`.
6. **Backfilling AI enrichment for the ~6,252 pre-existing jobs is deliberately deferred**, not
   silently folded into steady-state ingestion. A one-time backfill needs its own throttled,
   cost-aware pass (e.g. capped batch size per interval, or an explicit admin-triggered job with a
   dry-run cost estimate first) — exactly the kind of decision `ai_usage` cost tracking (a later
   phase) is meant to make routine instead of a surprise.
7. **Lesson:** whenever flipping a lazy/on-demand code path to eager/proactive, check the existing
   backlog size first — a "flag day" migration like this needs an explicit backfill plan, not the
   same trigger as steady-state per-item processing.

## Phase L: AI usage/cost tracking (pulled forward)

New `ai_usage` table + `internal/aiusage` package record every AI-worker operation's outcome
(operation, status, latency_ms, cache_hit, error_message). `aiclient.Client` gained a
`SetUsageRecorder` hook invoked via one `defer c.track(ctx, "op")(&err)` line per public method;
several hand-rolled HTTP call sites were consolidated onto the existing `postJSON` helper (now
operation-aware) in the process. `jobrequirements.Service.WithUsageTracking` records cache-hit
events directly, since a cache hit never reaches `aiclient` at all — this is exactly the signal
that would have caught the Phase A incident immediately instead of requiring a manual row-count
cross-check. v1 scope is latency/status/cache_hit only (no tokens/cost/user_id/job_id yet, since
aiclient's method signatures don't carry user/job IDs and the AI worker doesn't return token usage
yet) — a reasonable, honestly-scoped first cut given the incident that motivated it.

## Phase C: job lifecycle (ACTIVE/CLOSED)

`jobs.status` existed in the schema since Phase 12 but nothing ever transitioned it — every job
stayed ACTIVE forever. Added `CloseStaleJobs` (marks `(source, company_id)` jobs CLOSED when
`last_seen_at` predates the current poll's start) and made `UpsertJob`'s `ON CONFLICT` also reset
`status = 'ACTIVE'` so a re-listed job revives instead of staying closed. Deliberately **not**
applied to `ARBEITNOW`: its `MaxPages` cap means "not seen this poll" often just means "pushed past
the page cap by newer postings," not "actually closed" — applying closure detection there would
false-positive close jobs that are probably still live. Single-company sources (Greenhouse/Lever/
Ashby) fetch their full current listing every poll, so it's safe there.

## Phase B: cross-source job deduplication

Rather than the full `job_postings`/`canonical_jobs` table split (large schema+query rewrite
touching matching/tailoring/frontend, high risk for the same end result), implemented dedupe as a
self-referencing link: `jobs` gained `fingerprint` (normalized `company|title|remote_type` — uses
`remote_type` instead of raw location text since it's already normalized identically by every
connector, unlike free-text location which varies far more across sources) and a nullable
`canonical_job_id`. `Ingest()` links new rows to an existing canonical job sharing a fingerprint
from a DIFFERENT source; `ListJobs`/`CountJobs` filter `canonical_job_id IS NULL`. Not backfilled
for pre-existing jobs (empty fingerprint matches nothing) — same "no flag-day mass update" lesson
as Phase A/D.

## Phase E: semantic embeddings (pgvector)

Added `pgvector` (via the `pgvector/pgvector:pg16` image — see the critical incident below for why
this specific image swap was dangerous) and `jobs.embedding vector(1536)` + an HNSW cosine index.
The AI worker gained a generic `/v1/embeddings` endpoint (`text-embedding-3-small`) reused for both
job and future candidate-profile embeddings. `aiclient.Embed`, a new `EmbedWorker`, and
`Repository.SearchByEmbedding` (cosine-distance ANN query, feeding Phase G's semantic-retrieval
stage) round out the Go side. Eager embedding generation is scoped to newly-inserted jobs only
(same insert-only trigger as Phase D's enrichment, for the same cost-safety reason) — backfilling
embeddings for pre-existing jobs is deliberately deferred to its own throttled pass.

sqlc note: the `vector` column is nullable (most rows haven't been embedded yet), but
`pgvector-go.Vector` can't scan a SQL NULL and sqlc's `nullable: true` override flag didn't produce
a pointer type in practice. Fix: queries that don't need the embedding (`UpsertJob`, `GetJobByID`,
`ListJobs`, `FindCanonicalByFingerprint`) now use an explicit column list excluding
`embedding`/`embedding_model`/`embedded_at` instead of `SELECT *`/`RETURNING *`; only
`SearchJobsByEmbedding` (which filters `WHERE embedding IS NOT NULL`) selects it, where scanning a
non-nullable `pgvector.Vector` is safe.

### CRITICAL INCIDENT: Postgres image swap corrupted collation-dependent indexes

Switching `postgres:16-alpine` (musl libc) to `pgvector/pgvector:pg16` (Debian, glibc) while
reusing the same data volume corrupted every TEXT-column btree index built under the old libc's
collation ordering. This let genuine duplicates slip past UNIQUE constraints during concurrent
writes: 97 duplicate `jobs` rows (same `source, external_id`) and 20 duplicate `companies` rows
(same `normalized_name`) were created by the 5 concurrent background workers before detection. One
of the brand-new Phase B partial indexes (`jobs_fingerprint_idx`) was corrupted badly enough to
block writes entirely ("cannot find insert offset").

**Tell that gave it away**: a `duplicate key value violates unique constraint` error on an
`embed_job` task, whose only SQL is `UPDATE jobs SET embedding = ... WHERE id = $1` — a statement
that doesn't touch `source`/`external_id` at all. A failing statement that can't possibly cause the
constraint it's blamed for is the signature of index corruption, not an application bug.

**Fix** (verified at each step): stopped `api` to halt concurrent writers; dropped the actively
-corrupted indexes; deduplicated `companies` (merge by `normalized_name`, repoint
`jobs.company_id`/`job_sources.company_id` to the earliest row, delete losers); deduplicated `jobs`
(true same-source duplicates aren't the cross-source case `canonical_job_id` was built for, so
losers were deleted outright after confirming `job_requirements.job_id ON DELETE CASCADE` made that
safe); recreated the dropped indexes fresh; ran `REINDEX DATABASE applyforge` for everything else
(checked all other TEXT-based unique constraints — users email/google_id, sessions token_hash,
job_sources, skill_aliases, etc. — found zero additional duplicates, reindexed anyway since
ordering corruption can exist without yet having produced an observable one). Confirmed: no invalid
indexes remain, a manual duplicate INSERT is now correctly rejected, full test suite green, live
pipeline ran cleanly with zero failures afterward.

**Going forward**: never swap Postgres base images across libc families (alpine/musl <-> glibc) on
a live volume. This repo is now on `pgvector/pgvector:pg16` (glibc) — future Postgres image changes
must stay within glibc-based images, or migrate to ICU collation deliberately via dump/restore.

## Phase F: CandidateIntelligenceProfile

Candidate information was spread across `profile`, `preferences`, `candidate_skills`, and
`resume`/`resume_experiences` with no single compact representation for matching/ranking to read.
Added `candidate_profile_versions` (append-only; "current" = highest `version` per user) storing
an AI-synthesized summary: target roles, seniority, core vs secondary skills, transferable-skill
signals (skill/evidence/strength), domains, architecture strengths, leadership signals, experience
evidence, and a narrative summary - plus its own `embedding vector(1536)` for Phase G's
candidate-side semantic retrieval.

1. **AI worker**: new generic `POST /v1/candidates/profile` (`app/candidates/`), heuristic fallback
   is a straightforward pass-through/aggregation of the input (there's no meaningful deterministic
   substitute for genuine synthesis like inferring domains from bullet text - the fallback exists
   purely so the endpoint stays functional without `OPENAI_API_KEY`, same policy as every other
   AI-backed endpoint).
2. **Trigger**: `resume.ParseWorker` gained an optional `onParsed` callback (set via `SetOnParsed`,
   not a direct import) invoked after a resume successfully parses - `main.go` wires it to enqueue
   `build_candidate_profile`. Kept as a callback specifically so package `resume` doesn't need to
   import `candidateprofile` (which already imports `resume` for `ListForUser`/`ResumeProfile`,
   so the reverse import would cycle).
3. **`BuildWorker`** does both generation AND embedding in one background job (unlike jobs, where
   enrichment and embedding are separate job types) - simpler here since both steps are already
   triggered by the exact same event (a resume re-parse) with no independent trigger for either.
4. **Master skills come from `candidate_skills`, not the raw resume-parsed skill list** - it's the
   normalized/deduplicated/user-editable source of truth (which the resume parser itself populates
   in the first place), a more authoritative input than re-reading the coarser earlier-stage
   `ResumeProfile.Skills` again.
5. **Hit the "nil slice marshals to JSON null" bug again** (see repo memory `debugging.md`) - a
   user profile with no onboarding data yet (`profile.ErrNotFound`, leaving `Profile{}`'s slice
   fields nil) produced a 422 from the AI worker's `list[str] = Field(default_factory=list)`
   validation, since an explicit JSON `null` doesn't fall back to the default the way an *omitted*
   key does. Fixed with a generic `emptyIfNil[T any]` helper applied before both the AI-worker
   request and the Postgres `TEXT[]` insert (same underlying issue, two different serialization
   boundaries hit it independently).
6. **Not yet wired to preferences changes** - only resume re-parse triggers a rebuild for now;
   updating preferences alone doesn't yet enqueue one. Reasonable v1 scope since a resume re-parse
   is the highest-value trigger and preferences change far less often than resumes get re-uploaded.

Verified: go build/vet/test + ai-worker ruff/pytest all green. Live: enqueued a real
`build_candidate_profile` job for a user with an already-parsed resume - produced a genuine
AI-synthesized summary (correct core skills, an accurate narrative summary reflecting real resume
content) and a real embedding, end to end.

## Phase G: hard-filter -> vector-retrieval -> deterministic-score funnel

Ties the pre-existing deterministic scorer (`matching.Match`, already the "cheap deterministic
score" stage) together with Phase E/F's embeddings. `SearchJobsByEmbedding` now takes the same hard
-filter predicates as `ListJobs` (remote_type/employment_type/posted_after) in the SAME query as the
cosine-distance ranking, so stages A and B happen in one round trip instead of two. New
`matching.Service.Recommend(ctx, userID, limit)`: draws a generously-sized pool via semantic
retrieval (the pool isn't the final cut - deterministic scoring is), scores each via the existing
`Match()`, drops anything `!Eligibility.Eligible` (hard failures like excluded companies), sorts by
`TotalScore`. No dedicated automated test for `Recommend()` itself - it depends on `Match()`, which
depends on `jobrequirements`/`aiClient`, and this codebase's integration tests deliberately avoid
live AI calls; verified via Phase I's worker instead (see below), which calls it directly.

## Phase H: AI job-fit ranker

A deterministic score can't judge things like "is this skill gap actually easy to close" the way an
LLM given the right context can - this phase adds that judgment on top of (not instead of) Phase G's
funnel.

1. **New `internal/airank` package, deliberately separate from `matching`** - `matching`'s own doc
   comment states scoring is never delegated to an LLM; `airank` is consciously a distinct, later
   stage rather than blurring that boundary.
2. **AI worker**: `POST /v1/candidates/rank-jobs` (`app/candidates/ranking.py`) takes a candidate
   summary + a *batch* of jobs (not one call per job - the model can compare/prioritize across a
   batch, and it's far cheaper) and returns fit_score/interview_probability_score/career_alignment/
   skill_gap_severity/recommendation per job. Heuristic fallback is a simple threshold mapping off
   the deterministic score - a reasonable default, not a real substitute for the model's judgment.
3. **`airank.Service.Rank`** batches candidates in groups of 20, merges AI judgments back by job ID,
   and sorts by `fit_score` - falling back to `Result.TotalScore` for any job whose batch's AI call
   failed, so one transient AI failure never makes the whole ranked list disappear.
4. **The AI can override the deterministic ranking** - a job with a lower `TotalScore` but an
   easily-transferable gap can out-rank a higher-scored job with a fundamental mismatch, which is
   the whole point of layering judgment on top of arithmetic. Verified via a unit test with a fake
   AI response where the lower-deterministic-score job ranks first.

Verified: go build/vet/test all green, including new `airank` unit tests (httptest-backed, no real
network) covering batching, judgment merging, fit_score-driven reordering, and graceful fallback to
`TotalScore` when the AI call fails.

## Phase I: async precomputed recommendations

`/jobs` (or a future recommendations UI) shouldn't run the full funnel (hard filter -> semantic
retrieval -> deterministic score -> AI rerank) on every request. New `job_recommendations` table
materializes the ranked result once per profile change; reads become a simple indexed `SELECT`.

1. **`jobrecommendations.ComputeWorker`** runs Phase G's `Recommend()` + Phase H's `airank.Rank()`
   and calls `ReplaceForUser` (delete-then-insert, atomic enough for a small N<=50 set fully
   recomputed together every time - no piecemeal updates to reconcile).
2. **Trigger**: `candidateprofile.BuildWorker` gained an optional `onBuilt` callback (`SetOnBuilt`,
   same pattern as `resume.ParseWorker`'s `onParsed` - a callback instead of a direct import, since
   `jobrecommendations` already imports `candidateprofile`) invoked after a profile is built AND
   embedded; wired in `main.go` to enqueue `compute_recommendations`.
3. **`final_score`** is the AI `fit_score` when available, falling back to the deterministic
   `TotalScore` for any job whose AI ranking failed - both are stored, not just the winner, so it's
   possible to see later why a job ranked where it did.
4. **New `GET /recommendations`** (`jobrecommendations.Handlers`) - a fast, purely indexed read.
5. **Not yet wired**: a "new job arrives -> find candidates whose embedding is close -> recompute
   their recommendations" trigger in the other direction (would need a reverse-nearest-neighbor
   query against all candidate embeddings). Deferred - profile-change-triggered recompute is the
   higher-value v1 trigger, and this fan-out adds real complexity for a less common event.

### Debugging note: an empty-looking end-to-end test that wasn't actually a bug

Live verification initially produced zero recommendations. Traced through a temporary standalone
Go program (`cmd/debugrecommend`, deleted after use) that called each pipeline stage directly
against the real dev DB: `Recommend()` alone worked perfectly (6 real candidate jobs, correct
distances, correct eligibility) - the actual cause was that **the ai-worker container had never
been rebuilt after Phase H's ranking endpoint was added** (a separate rebuild step from the `api`
container, easy to forget since most of this session's changes only needed one or the other).
`POST /v1/candidates/rank-jobs` was returning 404, `airank.Service.Rank` correctly treated that as a
failed AI call and fell back to `TotalScore`-only ordering (working as designed) - the "bug" was an
un-rebuilt container, not application logic. Once rebuilt, live verification showed a genuinely
useful result: the deterministic scorer ranked "Account Maintenance Associate" highest (55, pure
keyword-overlap coincidence) while the AI reranker correctly recognized it's irrelevant to a backend
engineering candidate and ranked it last (10) - exactly Phase H's intended value.

Verified: go build/vet/test all green. Live: real `compute_recommendations` run via the actual
background worker (not just a debug script) produced 6 AI-ranked, persisted recommendations with
sensible reasoning, confirming Phases G, H, and I work together correctly end-to-end.






## Phase J/K: async multi-pass resume tailoring + AI critic/revision loop

1. **Extended, not replaced, `tailoring_runs.status`.** The CHECK constraint already had
   `PENDING/COMPLETED/FAILED` from the original synchronous design; migration
   `00034_tailoring_async_critic.sql` widens it to add `WRITING/EVALUATING/REVISING`, plus new
   `critic_result JSONB` and `revision_count INTEGER NOT NULL DEFAULT 0` columns.
2. **Reused the existing `GET /tailoring/{id}` handler as the polling endpoint** — it already
   re-fetches the run + suggestions fresh on every call, so no new polling API was needed; the
   frontend just needs to keep polling until `status` reaches `COMPLETED`/`FAILED`.
3. **`Tailor()` split into `CreateQueuedRun` (fast, synchronous, no AI call — just validates and
   inserts a PENDING row) + `ProcessRun` (the actual async worker body).** `handleTailor` now
   enqueues a `process_tailoring_run` background job and returns `202 Accepted` immediately instead
   of blocking on the AI call chain. `Tailor()` itself is kept as a thin synchronous wrapper
   (`CreateQueuedRun` + `ProcessRun`) for tests/manual use — no other code path calls it directly.
4. **Critic loop is capped at one revision** (`maxRevisions = 1`). `ProcessRun`: WRITING (suggest) →
   EVALUATING (critique) → if `recommend_regeneration && revisionCount < maxRevisions`: REVISING →
   WRITING again (critic feedback appended to the `Responsibilities` field sent to the AI, prefixed
   `"CRITIC FEEDBACK FROM PREVIOUS DRAFT (address this): "`) → EVALUATING again → store final
   `critic_result`/`revision_count` via `SetCritic` → persist suggestions → compute
   `alignmentAfter` → `CompleteRun`. Bounded to 1 revision to keep cost/latency predictable per the
   session's established AI-cost-safety principle.
5. **Critic AI-worker endpoint (`POST /v1/tailoring/critique`)** follows the same
   AI-then-heuristic-fallback pattern as every other ai-worker endpoint this session
   (`critique_heuristic` is deliberately conservative: never flags unsupported claims, never
   recommends regeneration, only computes missing keywords — so a critic-service outage degrades to
   "no revision, ship what we have" rather than blocking the pipeline).

### Live verification

Created a real `tailoring_runs` row (GROWTH mode) for an existing user/resume/job, enqueued
`process_tailoring_run` directly, and polled `tailoring_runs.status` every few seconds. Observed the
full intended transition: `PENDING → WRITING → EVALUATING → REVISING → WRITING → EVALUATING →
COMPLETED`, with `revision_count = 1` — the critic genuinely triggered a real second pass
(`recommend_regeneration: true`, missing keywords like "Java"/"C/C++"/"Leadership", ATS score 90).
Final `tailoring_suggestions` rows reflected the revised draft. Confirms Phases J and K work
end-to-end against real OpenAI calls, not just in isolation.

Verified: `go build/vet/test ./...` all green (including pre-existing `internal/tailoring` tests,
confirming the `Tailor()`→`CreateQueuedRun`/`ProcessRun` refactor didn't break anything). Both `api`
and `ai-worker` containers rebuilt together per the Phase I lesson.

**All 12 phases (A-L) of the original architecture overhaul are now complete.**
