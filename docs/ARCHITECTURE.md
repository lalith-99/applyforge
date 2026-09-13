# ApplyForge architecture

Current baseline and upgrade direction, reviewed September 13, 2026. The detailed
[code review and implementation design](ARCHITECTURE_REVIEW_2026-09-13.md) identifies remaining correctness,
coverage, AI-cost and application-execution gaps. Proposed components in that document are not all implemented.

## Runtime boundaries

| Component | Current responsibilities |
|---|---|
| Next.js web | Account/onboarding, jobs, recommendations, resume review/preview, application tracking, learning and analytics. |
| Go API | Authentication, business rules, source discovery/ingestion, matching, approval checks, database access and worker orchestration. |
| Go background workers | PostgreSQL queue consumers for ingestion, resume/profile processing, enrichment, embeddings, recommendation computation and tailoring. Run in the API process today. |
| Python FastAPI service | OpenAI structured requests and embeddings; heuristic fallbacks; PDF/DOCX text extraction and document generation. Called over HTTP by Go. |
| PostgreSQL / pgvector | Users, source graph, jobs, sponsor evidence, deterministic results, embeddings, recommendation read model, task queue and usage events. |
| S3-compatible storage | Resume uploads and generated documents; MinIO locally. |

## Job and recommendation flow

Verified source discovery feeds `company_source_registry` and `job_sources`. Direct ATS connectors and
optional broad providers produce raw postings. Go normalizes location/role/employment metadata, upserts
jobs, links cross-source duplicates, records sponsorship exclusions and queues enrichment for selected
new/changed dated U.S. jobs. Concrete `job_source_postings` membership now scopes closure to one board/tenant;
poll generations fence superseded workers, partial persistence publishes no closure, and suspicious empty
snapshots preserve the last known inventory. Source poll and coverage endpoints expose acquisition health.

Recommendations combine hard filters, semantic retrieval, an independent lexical pool, deterministic
scoring, company diversity, and AI ranking in groups of 20. Up to 20 recommendations are materialized per
user. Replacement now uses a transaction and per-user lock so a failed write preserves the old set.
This does not yet prevent an older computation from publishing after a newer candidate revision.

Hourly refresh and catalog-change debouncing already exist. Persisted fit-judgment caches and an enforced
AI spending gate do not yet exist. Review [AI_PIPELINE.md](AI_PIPELINE.md) before expanding volume.

## Resume and application flow

The user can review tailoring suggestions, attest to claims that require confirmation, generate a
version and download PDF/DOCX. The application API stores reusable answers and tracks statuses/events.
It does not submit to ATS systems. The proposed executor requires an immutable document/answers package,
scoped approval, ownership checks, idempotent intent creation and confirmation receipts; see the review.

## Design decisions

Retain the modular Go backend and PostgreSQL queue. Separate listing/detail/AI worker capacity before
adding infrastructure. Keep catalog acquisition independent of user-specific AI work. The first
source-observation migration now tracks tenant membership safely; separating the stable canonical job from
all source-provided fields remains follow-up work. Add a local
browser companion only for supported, permitted application flows; public ATS listing access is not
submission authorization.

Local startup learns sources from the catalog and a sponsor-prioritized public ATS inventory. It no
longer recreates the former fixed company list. This change does not delete existing sources or user data.
An empty installation still needs discovered/imported sources and sponsor evidence for the relevant views.

## Deployment and verification

Local development uses Docker Compose for PostgreSQL, MinIO, Go and Python; the web app runs separately.
See [DEPLOYMENT.md](DEPLOYMENT.md) for repository deployment configuration. This review did not inspect
an active production deployment.

CI defines Go tests with PostgreSQL/pgvector, Python tests, web lint/build and Compose validation.
Coverage and quality require additional corpus-based and source-lifecycle tests described in the review;
a green unit suite alone does not establish complete job coverage or reliable automated applications.
