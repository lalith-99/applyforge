# ApplyForge — Architecture

## System Overview

```
Next.js (apps/web)
      │  HTTPS/JSON
      ▼
Go API (apps/api)  ──────────────►  PostgreSQL (primary DB, job queue, scheduler state)
      │  HTTPS/JSON
      ▼
Python AI Worker (apps/ai-worker) ──► AI Provider (structured output only; currently a heuristic stand-in)
      │
      ▼
Object Storage (S3-compatible: Cloudflare R2 / AWS S3; MinIO locally)
```

Both the Go API and the Python AI worker read/write Object Storage directly (Go stores/serves resume source
files and generated documents; Python reads resume source files for extraction and writes generated PDFs/DOCX).

## Why this shape

* **Next.js** — modern DX, App Router, server components where useful, deployed to Cloudflare.
* **Go API** — owns all business/domain logic that must be reliable, transactional, and cheap to run:
  auth integration, users/profiles, job ingestion/normalization/dedup, deterministic matching, application
  tracking, scheduling, background job orchestration, analytics, rate limiting, authorization.
* **Python AI worker** — owns everything that benefits from the Python AI/document ecosystem: resume/JD
  parsing, transferable-skill reasoning, resume tailoring suggestions, Quick Prep, Defend This Bullet,
  learning plans, PDF/DOCX generation. Stateless from the Go API's point of view — it is called
  synchronously over HTTP for on-demand operations and is itself a consumer of the Postgres-backed job
  queue for expensive/batch work.
* **PostgreSQL** — single source of truth. Also used as the background job queue (`background_jobs` table,
  `SELECT ... FOR UPDATE SKIP LOCKED`) and scheduler persistence. No Redis/Kafka/RabbitMQ/Temporal in the MVP.
* **Object storage** — S3-compatible so Cloudflare R2 (initial) and AWS S3 (fallback) are interchangeable.

## Explicitly excluded from MVP

Kubernetes, Kafka, RabbitMQ, Elasticsearch, Temporal, Redis (unless a concrete need appears), vector database,
service mesh, event sourcing, dozens of microservices. See [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) §5, §62.

## Monorepo layout

```
apps/
  web/         Next.js + TypeScript frontend
  api/         Go backend (chi, pgx, sqlc, goose)
  ai-worker/   Python FastAPI AI/document service
packages/
  contracts/   Shared API contract artifacts (OpenAPI schema, generated types)
docs/          Architecture & product documentation (this folder)
infra/
  docker/      Local/shared Dockerfiles
  railway/     Railway deployment config/notes
  cloudflare/  Cloudflare Pages/Workers deployment notes
.github/workflows/  CI
docker-compose.yml  Local dev: Postgres + API + AI worker
Makefile            Common dev commands
```

## Cross-service contracts

The Go API is the only backend surface the frontend talks to. The Go API calls the Python AI worker
server-to-server over HTTP using a small internal client; the AI worker is never called directly from the
browser. This keeps authorization, rate limiting, and caching centralized in Go.

`packages/contracts` will hold the OpenAPI spec generated from the Go API and any shared TypeScript types
consumed by the frontend, once Phase 1+ introduces real endpoints.

## Environments

Local dev: Docker Compose (Postgres + api + ai-worker), web run via `pnpm dev` outside Docker for fast HMR.
Deployed: web → Cloudflare, api & ai-worker → Railway, Postgres → Neon, storage → Cloudflare R2.

## Status (through Phase 7)

The full core loop is now live end-to-end: sign up → upload resume (parsed via the ai-worker into
candidate skills + structured experience) → jobs are ingested hourly from real Greenhouse/Lever/Ashby boards
→ deterministic Job Match Score computed per job/user → Tailor Resume produces STRICT/GROWTH/MAX_MATCH
suggestions with a Resume Alignment Score → user approves/rejects suggestions. Object storage (MinIO
locally, S3-compatible in production) was added in Phase 2 for resume files; a Postgres-backed background
job queue (`internal/background`) processes resume parsing asynchronously.

**Not yet real AI**: resume/JD parsing and tailoring suggestions all use deterministic heuristic
implementations (no `AI_API_KEY` configured) — see AI_PIPELINE.md for why this is a documented scope
decision, not a shortcut taken silently.

Not yet built: Quick Prep / Defend This Bullet / Make Me Qualified (Phase 8), resume PDF/DOCX generation
(Phase 9), application tracking (Phase 10), analytics (Phase 11), and the Immigration-Aware Job Matching
sub-system described in MASTER_REQUIREMENTS.md (out of scope for "Phases 2-7" as enumerated).
