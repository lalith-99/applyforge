# ApplyForge

An AI-powered job search, resume tailoring, application tracking, and interview preparation platform.

> Go from job posting to interview-ready.

This repository is being built incrementally, phase by phase. See
[docs/MASTER_REQUIREMENTS.md](docs/MASTER_REQUIREMENTS.md) for the full product/engineering spec,
[docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) for phase status, and
[docs/DECISIONS.md](docs/DECISIONS.md) for what has actually been built and why.

**Status: Phases 0-12 complete (MVP feature-complete).** A user can sign up, upload a master resume (parsed
into structured experience + candidate skills), browse real jobs ingested hourly from Greenhouse/Lever/Ashby,
see a deterministic Job Match Score with matched/missing/transferable skills, tailor their resume
(STRICT/GROWTH/MAX_MATCH) and approve/reject AI-suggested changes, see a Resume Alignment Score, get Quick
Prep/Defend This Bullet/Make Me Qualified/Interview Readiness/learning-plan guidance for a job, generate and
download a tailored PDF/DOCX resume version, track applications through a Kanban/table board with a full
status-change history, view conversion-funnel/response-rate/match-score analytics, and delete a single
resume or their entire account (which cascades everywhere in the database and cleans up object storage).
Resume/JD parsing, tailoring suggestions, and Quick Prep/learning-plan content currently use deterministic
heuristics rather than a real LLM (no `AI_API_KEY` configured — see docs/AI_PIPELINE.md). See
[docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) and [docs/DECISIONS.md](docs/DECISIONS.md) for
the full per-phase breakdown and known scope limitations.

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

# Postgres + MinIO + Go API + Python AI worker
make docker-up

# Apply database migrations (first time, or after pulling new migrations)
make migrate

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
| `make docker-up`     | Same as above                                              |
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

See [docs/DECISIONS.md](docs/DECISIONS.md) for the full, phase-by-phase list. In summary: no real AI
provider is wired in (heuristic stand-ins throughout, documented not hidden); the Immigration-Aware Job
Matching sub-system (DOL data ingestion) is not built; rate limiting is a simple in-memory per-IP limiter
(not distributed — would need a shared store like Redis behind multiple API replicas); and there's no
admin/ops dashboard beyond the per-user `/analytics` page.
