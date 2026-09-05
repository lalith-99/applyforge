# ApplyForge — API

## Versioning

All product endpoints are served under `/api/v1` by the Go API (`apps/api`). The Python AI worker
(`apps/ai-worker`) exposes an internal-only API consumed solely by the Go API, not by the browser.

## Implemented endpoints (through Phase 12 — feature-complete)

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

Learning (`internal/learning`):

```
GET    /api/v1/skills/{skill}/quick-prep        Quick Prep module for a skill (cached; personalized
                                                 transferable_from at request time, never cached)
POST   /api/v1/defend-bullet                    body: {bullet_text, skills}; likely interview questions
                                                 for a resume bullet (not cached; bullet text/skills vary)
POST   /api/v1/jobs/{id}/learning-plan           learning plan for the caller's missing skills on a job
POST   /api/v1/jobs/{id}/make-me-qualified       aggregated current/target match, high/low-value gaps,
                                                  Interview Readiness score, and a learning plan
```
Resume versions (`internal/resumeversion`):

```
POST   /api/v1/resumes/{id}/versions             body: {job_id?, tailoring_run_id?}; merges any
                                                  approved/edited suggestions from the given run onto
                                                  the base resume and generates PDF + DOCX
GET    /api/v1/resumes/{id}/versions             list versions for a base resume, newest first
GET    /api/v1/resume-versions/{id}              version detail (content, scores, storage keys)
GET    /api/v1/resume-versions/{id}/download     ?format=pdf|docx (default pdf); streams the file
```

Applications (`internal/applications`):

```
POST   /api/v1/applications                       body: {job_id, resume_version_id?, match_score?};
                                                   idempotent upsert on (user_id, job_id) — "Save Job"
GET    /api/v1/applications                       list, joined with job display fields, newest-updated first
GET    /api/v1/applications/{id}
PATCH  /api/v1/applications/{id}                  body: {status?, notes?, next_action?}; a status change
                                                   logs an application_events row (from/to status)
GET    /api/v1/applications/{id}/events           status-change history, oldest first
GET    /api/v1/application-answers                zero-value defaults if none saved yet, not a 404
PATCH  /api/v1/application-answers
```

Analytics (`internal/analytics`):

```
GET    /api/v1/analytics/dashboard                jobs discovered, applications tracked, tailoring runs,
                                                   high matches, conversion funnel, response rate,
                                                   average match score
```

Account (`internal/account`):

```
DELETE /api/v1/resumes/{id}                       deletes the resume's uploaded file + any generated
                                                   PDF/DOCX version files from object storage, then the
                                                   DB row (cascades resume_experiences/resume_versions)
DELETE /api/v1/account                            deletes every resume's object-storage files, then the
                                                   user row (cascades everywhere else in the schema);
                                                   also clears the af_session cookie
```

## Rate limiting

All `/api/v1` routes are rate limited per client IP (in-memory, fixed-window, not distributed — see
DECISIONS.md Phase 12): `/api/v1/auth/*` at 20 requests/minute, everything else at 300 requests/minute.
Exceeding the limit returns `429 {"error": "too many requests"}`.

## Planned endpoint surface

None — the endpoint surface for Phases 0-12 is complete (this was the final planned phase).

OpenAPI docs are deferred until the endpoint surface stabilizes further — not worth generating/maintaining
yet (tracked as technical debt, see DECISIONS.md).

## Operational endpoints

* Go API: `GET /health` (liveness), `GET /ready` (liveness + DB connectivity).
* Python AI worker: `GET /health`, `GET /ready`.
