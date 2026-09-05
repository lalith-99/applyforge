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

## Phase 0 status

No auth, upload, or storage code exists yet, so most controls above are not yet applicable. What Phase 0
does establish: no secrets committed to the repo (`.env.example` only), `.gitignore` covers `.env`, and CI
does not print secret values.
