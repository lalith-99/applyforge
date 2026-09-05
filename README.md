# ApplyForge

An AI-powered job search, resume tailoring, application tracking, and interview preparation platform.

> Go from job posting to interview-ready.

This repository is being built incrementally, phase by phase. See
[docs/MASTER_REQUIREMENTS.md](docs/MASTER_REQUIREMENTS.md) for the full product/engineering spec,
[docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) for phase status, and
[docs/DECISIONS.md](docs/DECISIONS.md) for what has actually been built and why.

**Status: Phase 0 (repository scaffolding) complete.** No product features exist yet — only three runnable
services with health checks, docs, and CI/local-dev plumbing.

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
* Go 1.24+
* Python 3.12+ (a `python3.13` or newer interpreter works fine)
* Docker + Docker Compose

## Local development

```bash
cp .env.example .env

# Postgres + Go API + Python AI worker
make docker-up

# Frontend (run separately for fast HMR)
cd apps/web && pnpm install && pnpm dev
```

Once running:

* Web: http://localhost:3000
* API: http://localhost:8080/health, http://localhost:8080/ready
* AI worker: http://localhost:8000/health, http://localhost:8000/ready

Stop the backing services with `make docker-down`.

## Commands

| Command            | Description                                              |
|---------------------|-----------------------------------------------------------|
| `make dev`           | Start Postgres + api + ai-worker via Docker Compose        |
| `make docker-up`     | Same as above                                              |
| `make docker-down`   | Stop and remove the Docker Compose stack                   |
| `make build`         | Build all three services                                   |
| `make lint`          | Lint Go, Python, and web code                               |
| `make fmt`           | Format Go and Python code                                   |
| `make test`          | Run Go and Python test suites                               |
| `make migrate`       | Run database migrations (no-op until Phase 1)               |
| `make seed`          | Load fake development seed data (no-op until Phase 1)       |

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

## Next phase

Phase 1 (database foundation, authentication, user profiles, job preferences, onboarding) is not started.
See [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md).
