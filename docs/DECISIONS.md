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

* Auth provider specifics (NextAuth vs custom) — Phase 1.
* sqlc/goose migration structure — Phase 1.
* AIProvider concrete vendor — Phase 4.
* Object storage client library choice (Go + Python) — Phase 2.
