# ApplyForge

An AI-powered job search, resume tailoring, application tracking, and interview preparation platform.

> Go from job posting to interview-ready.

This repository is being built incrementally, phase by phase. See
[docs/MASTER_REQUIREMENTS.md](docs/MASTER_REQUIREMENTS.md) for the full product/engineering spec,
[docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) for phase status, and
[docs/DECISIONS.md](docs/DECISIONS.md) for what has actually been built and why.

**Current implementation:** account/onboarding, resume parsing and review, dynamic job-source discovery,
U.S. software job browsing, deterministic matching, hybrid retrieval, AI reranking, tailored PDF/DOCX
resumes, application tracking and interview preparation. OpenAI integrations are available when configured;
several operations fall back to deterministic heuristics. PostgreSQL stores the catalog, queue, sponsor
evidence and recommendation read model.

**Architecture upgrade:** see [the September 13 code review and design](docs/ARCHITECTURE_REVIEW_2026-09-13.md)
for confirmed flaws, source coverage improvements, an explicit monthly AI cost model, and the design for
user-approved application submission. Applications are currently tracked manually; automatic submission,
immutable submission approval packages and enforced AI budgets remain implementation work.

## Repository layout

```
apps/
  web/         Next.js (TypeScript, App Router, Tailwind CSS)
  api/         Go API (chi router, pgx, slog)
  ai-worker/   Python AI/document worker (FastAPI)
packages/
  contracts/   Shared API contracts (populated starting Phase 1)
docs/          Product & architecture documentation
infra/         Deployment configuration (Docker/Railway/Cloudflare)
```

## Prerequisites

* Node.js 20+ and `pnpm` (`npm install -g pnpm`)
* Go 1.25+ (toolchain auto-upgrades to 1.26 locally; Docker image and CI use 1.26)
* Python 3.12+ (a `python3.13` or newer interpreter works fine)
* Docker + Docker Compose
* `goose` and `sqlc` on your `PATH` if you need to change migrations/queries:
  `go install github.com/pressly/goose/v3/cmd/goose@latest && go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

## Local development

```bash
cp .env.example .env

# Postgres + MinIO + migrations + Go API + Python AI worker.
# Docker Compose waits for migrations to finish before starting the API.
make docker-up

# Frontend (run separately for fast HMR)
cd apps/web && pnpm install && pnpm dev
```

Once running:

* Web: http://localhost:3000 — sign up at `/signup`, onboarding at `/onboarding`, resumes at `/resume`,
  jobs at `/jobs`, tailoring at `/jobs/{id}/tailor`, applications at `/applications`, analytics at
  `/analytics`
* API: http://localhost:8080/health, http://localhost:8080/ready
* AI worker: http://localhost:8000/health, http://localhost:8000/ready
* MinIO console: http://localhost:9001 (user `applyforge` / password `applyforge123` locally)

To see real jobs, trigger an initial sync (the scheduler also runs hourly automatically):
```bash
curl -X POST http://localhost:8080/api/v1/admin/job-sources/sync -b <(curl -s -c - -X POST \
  http://localhost:8080/api/v1/auth/signup -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"supersecret1"}' -o /dev/null)
```

Note: docker-compose maps Postgres to host port **5433** (not 5432) to avoid clashing with other local
Postgres instances — see `.env.example` and DECISIONS.md.

Stop the backing services with `make docker-down`.

## Commands

| Command            | Description                                              |
|---------------------|-----------------------------------------------------------|
| `make dev`           | Start Postgres + api + ai-worker via Docker Compose        |
| `make docker-up`     | Start backing services, run migrations, then API/AI worker |
| `make docker-down`   | Stop and remove the Docker Compose stack                   |
| `make build`         | Build all three services                                   |
| `make lint`          | Lint Go, Python, and web code                               |
| `make fmt`           | Format Go and Python code                                   |
| `make test`          | Run Go and Python test suites                               |
| `make migrate`       | Apply goose database migrations                             |
| `make seed`          | Load fake development seed data (no-op until a later phase) |

### Per-service setup (without Docker)

**Go API**
```bash
cd apps/api
go run ./cmd/api
```

**Python AI worker**
```bash
cd apps/ai-worker
python3.13 -m venv .venv && ./.venv/bin/pip install -r requirements.txt -r requirements-dev.txt
./.venv/bin/uvicorn app.main:app --reload --port 8000
```

**Web**
```bash
cd apps/web
pnpm install && pnpm dev
```

## Documentation

* [PRODUCT.md](docs/PRODUCT.md) — product vision, positioning, MVP success criteria
* [ARCHITECTURE.md](docs/ARCHITECTURE.md) — system architecture, monorepo layout
* [DATABASE.md](docs/DATABASE.md) — schema conventions, planned tables
* [API.md](docs/API.md) — API surface (planned + implemented)
* [AI_PIPELINE.md](docs/AI_PIPELINE.md) — AI provider architecture, cost controls
* [MATCHING_ENGINE.md](docs/MATCHING_ENGINE.md) — deterministic scoring design
* [RESUME_TAILORING.md](docs/RESUME_TAILORING.md) — tailoring modes and approval flow
* [JOB_INGESTION.md](docs/JOB_INGESTION.md) — job sources, dedup, freshness, scheduler
* [SECURITY.md](docs/SECURITY.md) — security baseline
* [DEPLOYMENT.md](docs/DEPLOYMENT.md) — deployment topology
* [IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) — phase-by-phase plan and status
* [DECISIONS.md](docs/DECISIONS.md) — architectural decisions log

## Current scope limitations

The latest [architecture review](docs/ARCHITECTURE_REVIEW_2026-09-13.md) distinguishes shipped behavior from
remaining work. Critical follow-ups include publication-time provenance, tenant-scoped job closure,
strict eligibility semantics, durable AI result caches and spend limits, and receipt-backed application
execution. Real OpenAI integration, DOL sponsor evidence, source discovery and AI usage recording exist;
older entries in DECISIONS.md describe historical milestones rather than the complete current state.
