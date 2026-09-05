# ApplyForge — Security

## Data sensitivity

Resumes and candidate information are treated as sensitive personal data. Every user-owned resource requires
authorization at the API layer (no security-by-obscurity via unguessable IDs).

## Baseline controls (introduced as each phase adds the relevant surface)

* Secure authentication (email + Google OAuth), secure cookies, CSRF protection where applicable.
* Upload limits and MIME validation for resume uploads (PDF/DOCX only).
* Signed, time-limited object storage URLs — never public buckets.
* Rate limiting on all mutating and AI-backed endpoints.
* Parameterized SQL only (sqlc-generated queries; no string-built SQL).
* Secrets via environment variables / platform secret managers — never committed, never logged.
* Safe, generic error messages to end users; detailed errors only in structured server logs.
* Account deletion and resume deletion supported end-to-end (including object storage cleanup).

## Never logged

Passwords, access tokens, full raw resume text, private application answers, object-storage credentials, AI
provider keys.

## Immigration data handling

Government filing data (DOL LCA/PERM) and AI-derived immigration assessments are evidentiary, not
authoritative claims. The product must never state a guarantee ("this company will file your I-140") without
current, explicit, role-specific evidence — see [MASTER_REQUIREMENTS.md](MASTER_REQUIREMENTS.md) for full
wording rules. This is a product-accuracy/legal-risk concern as much as a security one.

## Phase 1 status

Implemented:

* Passwords hashed with bcrypt (`golang.org/x/crypto/bcrypt`, default cost); never stored or logged in
  plaintext.
* Sessions are opaque random tokens (32 bytes, `crypto/rand`); only a SHA-256 hash is persisted, so a
  database leak alone cannot be used to forge sessions.
* Session cookie (`af_session`) is `HttpOnly`, `SameSite=Lax`, and `Secure` in production
  (`ENVIRONMENT=production`); non-secure only for local HTTP development.
* Google OAuth uses a random per-attempt `state` value stored in a short-lived `HttpOnly` cookie and
  compared on callback (CSRF protection for the OAuth flow).
* Every `/profile` and `/preferences` route requires a valid session (`auth.RequireAuth` middleware); there
  is no way to read/write another user's data since the user id always comes from the resolved session, not
  from client input.
* CORS is restricted to `WEB_BASE_URL` with credentials enabled — no wildcard origins.
* Generic error messages are returned to clients (`httpx.WriteError`); Go's `slog` logs detailed errors
  server-side only.
* No secrets committed; `GOOGLE_CLIENT_SECRET`, `AUTH_SECRET`, etc. are read from the environment only.

Not yet implemented (tracked as technical debt): rate limiting, account deletion, resume deletion (no
resumes exist yet), email verification enforcement, CSRF protection for non-OAuth mutating requests (the
session cookie is `SameSite=Lax`, which mitigates most cross-site POST forgeries, but an explicit CSRF token
has not been added). These are appropriate to defer until Phase 12 (production hardening) unless a concrete
risk emerges sooner.
