# ApplyForge Browser Companion

The browser companion is the local execution surface for an **already reviewed and explicitly approved** ApplyForge application package. It never receives the user's ApplyForge session cookie and cannot browse or mutate arbitrary account data.

## Current support

- **Greenhouse**: fill known fields, attach the exact approved resume PDF, stop on missing required fields/CAPTCHA/login, then use the fenced final-submit flow.
- **Lever**: same behavior as Greenhouse.
- **Other ATS sites**: fill known fields and attach the approved resume when possible, but **do not auto-click final submit**. The user completes unsupported questions/submission manually.

The extension never attempts to solve CAPTCHA, bypass authentication, invent answers, or automatically retry an uncertain real-world submission.

## Local installation

1. Open Chrome or Edge extension management.
2. Enable **Developer mode**.
3. Choose **Load unpacked**.
4. Select `apps/browser-companion` from the ApplyForge repository.
5. Keep the extension enabled while using the **Review & apply** flow in ApplyForge.

The current manifest requests broad host access because job applications can live on many employer-owned domains. The extension does nothing on a site unless there is an active, short-lived ApplyForge handoff whose approved destination origin exactly matches the current page.

## Execution contract

1. The user moves an application to `READY_TO_APPLY` and opens **Review & apply**.
2. ApplyForge builds an immutable package from the server-owned job, exact job-scoped resume version, and saved application answers.
3. The user explicitly approves that exact package.
4. ApplyForge creates one idempotent submission intent.
5. The web app mints a random 20-minute companion capability bound to that one intent. Only its SHA-256 hash is stored server-side.
6. The web app passes the capability and exact reviewed PDF to the extension. The bearer stays in the extension service worker; page scripts do not receive it.
7. On the approved ATS origin, the companion fills deterministic known fields and checks for unsupported required questions/CAPTCHA/login.
8. Immediately before a supported final submit, the companion claims a five-minute lease and receives a monotonically increasing fencing generation, then calls `begin`.
9. A successful ATS confirmation is sent as a receipt. The server atomically marks the submission `CONFIRMED` and the tracked application `APPLIED`.
10. If the companion crossed the irreversible boundary but cannot prove success, it records `UNCERTAIN`. ApplyForge never retries that automatically. The user must reconcile it as **Verified applied** or **Verified not submitted**.

## Companion API

Session-authenticated handoff minting:

- `POST /api/v1/submission-intents/{id}/companion-handoff`

Intent-capability API (header `X-ApplyForge-Companion-Token`):

- `GET /api/v1/companion/submissions/{id}`
- `POST /api/v1/companion/submissions/{id}/claim`
- `POST /api/v1/companion/submissions/{id}/begin`
- `POST /api/v1/companion/submissions/{id}/confirm`
- `POST /api/v1/companion/submissions/{id}/uncertain`

## Development validation

CI parses the manifest and runs `node --check` on `background.js` and `content.js`. The API integration suite validates capability scoping, fencing, receipt confirmation, token revocation, and uncertain reconciliation against PostgreSQL.
