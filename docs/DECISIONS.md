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
