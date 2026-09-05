# ApplyForge — API

## Versioning

All product endpoints are served under `/api/v1` by the Go API (`apps/api`). The Python AI worker
(`apps/ai-worker`) exposes an internal-only API consumed solely by the Go API, not by the browser.

## Implemented endpoints (through Phase 7)

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

Resumes (`internal/resume`):

```
POST   /api/v1/resumes                  multipart upload (PDF/DOCX, 10MB limit); enqueues background parsing
GET    /api/v1/resumes                  list the caller's resumes
GET    /api/v1/resumes/{id}             detail incl. parsed_profile + experiences once PARSED
```

Jobs (`internal/jobs`):

```
GET    /api/v1/jobs                      paginated list; filters: search, remote_type, employment_type,
                                          posted_within (Go duration, e.g. 24h), sort (newest|salary)
GET    /api/v1/jobs/{id}                 detail; includes parsed requirements (lazily parsed + cached)
POST   /api/v1/admin/job-sources/sync    manually trigger a poll of all configured job sources
```

Matching (`internal/matching`):

```
GET    /api/v1/jobs/{id}/match            deterministic Job Match Score, Opportunity Score,
                                           Current/Target Profile Match (computed + cached per user)
```

Tailoring (`internal/tailoring`):

```
POST   /api/v1/jobs/{id}/tailor                          body: {resume_id, mode}; creates a TailoringRun
GET    /api/v1/tailoring/{id}                             run detail + suggestions
PATCH  /api/v1/tailoring/{id}/suggestions/{suggestionId}  body: {status, edited_text?}
POST   /api/v1/tailoring/{id}/approve-all                 flips all PENDING suggestions to APPROVED
```

## Planned endpoint surface (later phases)

```
POST   /api/v1/jobs/{id}/save
POST   /api/v1/jobs/{id}/make-me-qualified
POST   /api/v1/jobs/{id}/learning-plan
GET    /api/v1/skills/{skill}/quick-prep
POST   /api/v1/resume-bullets/{id}/defend
POST   /api/v1/applications
GET    /api/v1/applications
PATCH  /api/v1/applications/{id}
GET    /api/v1/analytics/dashboard
```

OpenAPI docs are deferred until the endpoint surface stabilizes further — not worth generating/maintaining
yet (tracked as technical debt, see DECISIONS.md).

## Operational endpoints

* Go API: `GET /health` (liveness), `GET /ready` (liveness + DB connectivity).
* Python AI worker: `GET /health`, `GET /ready`.
