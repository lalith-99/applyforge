# ApplyForge — API

## Versioning

All product endpoints are served under `/api/v1` by the Go API (`apps/api`). The Python AI worker
(`apps/ai-worker`) exposes an internal-only API consumed solely by the Go API, not by the browser.

## Implemented endpoints (Phase 1)

Authentication (`internal/auth`), all under `/api/v1/auth`:

```
POST   /api/v1/auth/signup              email+password signup, sets af_session cookie
POST   /api/v1/auth/login               email+password login
POST   /api/v1/auth/logout              invalidates the current session
GET    /api/v1/auth/session             current user (requires auth)
GET    /api/v1/auth/google/start        redirects to Google's consent screen
GET    /api/v1/auth/google/callback     completes Google OAuth, sets af_session, redirects to /onboarding
```

Profile and preferences (require auth via the `af_session` cookie):

```
GET    /api/v1/profile
PATCH  /api/v1/profile
GET    /api/v1/preferences
PATCH  /api/v1/preferences
```

`GET` returns sensible zero-value defaults (not a 404) when the user hasn't saved a profile/preferences yet,
so the onboarding UI can render immediately. `PATCH` performs a full upsert (whole-object replace, not a
partial merge) — the frontend always sends the complete form state.

Authorization: every route except `/auth/*` is mounted behind `auth.RequireAuth`, which resolves the
`af_session` cookie to a user or returns `401`. CORS is restricted to `WEB_BASE_URL` with credentials
enabled.

## Planned endpoint surface (later phases)

```
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

OpenAPI docs are deferred until the endpoint surface stabilizes further — not worth generating/maintaining
for 8 endpoints (tracked as technical debt, see DECISIONS.md).

## Operational endpoints

* Go API: `GET /health` (liveness), `GET /ready` (liveness + DB connectivity).
* Python AI worker: `GET /health`, `GET /ready`.
