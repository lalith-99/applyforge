# ApplyForge — API

## Versioning

All product endpoints are served under `/api/v1` by the Go API (`apps/api`). The Python AI worker
(`apps/ai-worker`) exposes an internal-only API consumed solely by the Go API, not by the browser.

## Planned endpoint surface (implemented incrementally, not present in Phase 0)

```
POST   /api/v1/auth/...
GET    /api/v1/profile
PATCH  /api/v1/profile
GET    /api/v1/preferences
PATCH  /api/v1/preferences
POST   /api/v1/resumes
GET    /api/v1/resumes
GET    /api/v1/resumes/{id}
GET    /api/v1/jobs
GET    /api/v1/jobs/{id}
GET    /api/v1/jobs/{id}/match
POST   /api/v1/jobs/{id}/save
POST   /api/v1/jobs/{id}/tailor
POST   /api/v1/jobs/{id}/make-me-qualified
POST   /api/v1/jobs/{id}/learning-plan
GET    /api/v1/tailoring/{id}
PATCH  /api/v1/tailoring/{id}/suggestions/{suggestionId}
POST   /api/v1/tailoring/{id}/approve-all
GET    /api/v1/skills/{skill}/quick-prep
POST   /api/v1/resume-bullets/{id}/defend
POST   /api/v1/applications
GET    /api/v1/applications
PATCH  /api/v1/applications/{id}
GET    /api/v1/analytics/dashboard
```

OpenAPI docs will be generated from the Go handlers as real endpoints land, starting Phase 1.

## Phase 0 status

Only operational endpoints exist today:

* Go API: `GET /health` (liveness), `GET /ready` (liveness + DB connectivity).
* Python AI worker: `GET /health`, `GET /ready`.

No `/api/v1/*` business endpoints exist yet.
