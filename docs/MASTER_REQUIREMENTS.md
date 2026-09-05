# APPLYFORGE — MASTER PRODUCT + ENGINEERING REQUIREMENTS

You are acting as:

* Principal Software Architect
* Senior Go Backend Engineer
* Senior Python AI Engineer
* Senior Next.js/TypeScript Engineer
* Product Designer
* DevOps Engineer
* Security Engineer
* QA Engineer

Your task is to design and build a production-quality SaaS product called **ApplyForge**.

Do NOT treat this as a demo, hackathon project, or basic CRUD application.

At the same time, do NOT overengineer the MVP.

Build a clean, modular system that can initially run cheaply for a small number of users and evolve later as usage increases.

---

## 1. PRODUCT VISION

ApplyForge is an AI-powered job search, resume tailoring, application tracking, and interview preparation platform.

The core workflow is:

JOB DISCOVERY
→ JOB MATCHING
→ RESUME TAILORING
→ USER APPROVAL
→ RAPID SKILL PREP
→ APPLICATION
→ INTERVIEW PREPARATION
→ APPLICATION TRACKING
→ ANALYTICS

Primary product promise:

> Go from a newly posted job to an optimized, application-ready resume in minutes.

Secondary promise:

> If the job requires technologies the candidate is unfamiliar with, explain exactly what they need to learn to discuss those technologies intelligently during an interview.

The platform should help candidates pursue roles they are reasonably capable of growing into rather than rejecting roles simply because every keyword is not already present on their resume.

---

## 2. IMPORTANT PRODUCT PHILOSOPHY

The system must distinguish between:

* what already exists on the user's master resume
* skills discovered through additional user input
* transferable skills
* new target skills suggested because of a job description
* AI-introduced resume suggestions
* user-approved additions

AI proposes changes. The user makes the final decision.

The user must be able to:

* approve a suggested skill immediately
* approve a suggested bullet immediately
* edit a suggestion
* reject it
* learn about the skill first
* ask AI how to defend the bullet
* add the change directly to the final tailored resume

Do NOT force users to complete: courses, projects, quizzes, timers, learning plans — before allowing approval.

Learning is optional. The product may recommend preparation, but the user owns the final decision.

However, never automatically fabricate: employers, employment dates, job titles, degrees, certifications, numerical metrics, security clearances, compensation, locations, named projects that never existed.

If AI proposes a materially new claim, surface that clearly before approval.

---

## 3. PRODUCT POSITIONING

Primary tagline: **Go from job posting to interview-ready.**

Main workflow: Discover → Match → Tailor → Learn → Apply → Prepare → Track

The system is NOT primarily marketed as "another AI resume builder". It should feel like: **an AI-powered operating system for the entire job search.**

---

## 4. MVP TECHNOLOGY STACK

### Frontend

* Next.js, TypeScript, App Router
* Tailwind CSS, shadcn/ui
* TanStack Query, Zod, React Hook Form
* Recharts when useful

### Main Backend (Go)

* Go 1.24+, chi router, net/http
* pgx, sqlc, goose migrations
* slog structured logging

Responsibilities: authentication integration, users, profiles, job preferences, job ingestion, job normalization, job deduplication, deterministic job matching, application tracking, scheduling, background task orchestration, API, analytics, rate limiting, authorization.

### AI / Document Service (Python)

* Python 3.12+, FastAPI, Pydantic
* pytest, Ruff, mypy or pyright
* PyMuPDF, python-docx, selectable-text PDF generation library

Responsibilities: resume extraction, structured resume parsing, job-description parsing, skill-gap reasoning, transferable-skill analysis, resume tailoring, quick interview preparation, "Defend This Bullet", learning-plan generation, DOCX/PDF generation.

### Database

PostgreSQL — primary database, background job queue, scheduler persistence, analytics source.

### Object Storage

S3-compatible storage, compatible with Cloudflare R2 / AWS S3.

### Initial deployment

Frontend: Cloudflare. Backend: Railway. AI worker: Railway. Database: Neon PostgreSQL. Storage: Cloudflare R2. Repository: GitHub. CI/CD: GitHub Actions.

---

## 5. THINGS WE MUST NOT ADD TO MVP

Do NOT introduce: Kubernetes, Kafka, RabbitMQ, Elasticsearch, Temporal, Redis (unless a real requirement appears), vector database, service mesh, event sourcing, dozens of microservices.

Initial architecture should remain:

```
Next.js → Go API → PostgreSQL
Go API → Python AI Service
Python AI Service → AI Provider
Both → Object Storage
PostgreSQL → Background jobs
```

Keep infrastructure cheap and understandable.

---

## 6. MONOREPO

```
/
apps/
  web/
  api/
  ai-worker/
packages/
  contracts/
docs/
  PRODUCT.md
  ARCHITECTURE.md
  DATABASE.md
  API.md
  AI_PIPELINE.md
  MATCHING_ENGINE.md
  RESUME_TAILORING.md
  JOB_INGESTION.md
  SECURITY.md
  DEPLOYMENT.md
  IMPLEMENTATION_PLAN.md
  DECISIONS.md
infra/
  docker/
  railway/
  cloudflare/
.github/
  workflows/
docker-compose.yml
Makefile
README.md
.env.example
```

---

## 7. DESIGN SYSTEM

Visual inspiration: Linear, Vercel, modern developer dashboards, premium SaaS products, modern job-search platforms. Do not copy proprietary assets or layouts.

Design characteristics: clean, modern, professional, high information density, generous spacing, subtle borders, rounded cards, responsive, accessible, fast, light mode, dark mode.

Navigation: Dashboard, Jobs, Resume, Applications, Learning, Analytics, Profile, Settings.

---

## 8. LANDING PAGE

Hero headline: **Stop applying with the same resume.**
Alternative supporting line: **Find better jobs. Tailor faster. Prepare smarter.**

Description: ApplyForge discovers high-fit opportunities, analyzes job descriptions, tailors your resume, identifies skill gaps, prepares you for unfamiliar technologies, and tracks your applications.

Primary CTA: Find My Matches. Secondary CTA: See How It Works.

Sections: 1. Product workflow 2. Smart job discovery 3. Job Match Score 4. AI resume tailoring 5. Skill-gap intelligence 6. Quick interview preparation 7. Application tracking 8. Analytics 9. Responsible AI disclosure 10. Pricing placeholder 11. FAQ 12. Final CTA.

Never add: fake testimonials, fake companies, fake usage numbers, fake success statistics.

---

## 9. AUTHENTICATION

Provide managed-auth-friendly architecture. Support email and Google OAuth. Keep provider abstraction reasonable. Every user-owned resource must require authorization.

---

## 10. ONBOARDING

**Personal:** first name, last name, email, city, state, country.

**Career:** primary target titles, alternative target titles, seniority, years of experience, preferred industries, preferred technologies, desired compensation.

**Job preferences:** remote, hybrid, onsite, preferred locations, willingness to relocate, full-time, contract, internship, minimum salary.

**Job restrictions:** excluded companies, excluded locations, excluded industries, clearance constraints, sponsorship/work authorization preferences.

Do not use protected demographic attributes for job ranking.

---

## 11. MASTER RESUME

Allow PDF upload, DOCX upload.

Pipeline: UPLOAD → STORE → TEXT EXTRACTION → AI STRUCTURED PARSING → USER REVIEW → MASTER PROFILE.

ResumeProfile: contact, summary, skills, experiences, projects, education, certifications.

Experience: id, company, title, start_date, end_date, location, bullets, detected_skills, technologies, metrics, concepts.

Store the original document permanently unless the user deletes it. Never modify the master resume when creating tailored versions.

---

## 12. CANDIDATE SKILL MODEL

CandidateSkill fields: id, user_id, normalized_name, display_name, category, proficiency, source, status, created_at, updated_at.

Status: VERIFIED_PROFESSIONAL, VERIFIED_PROJECT, FAMILIAR, LEARNING, TARGET_SKILL, USER_APPROVED, UNKNOWN.

Sources: MASTER_RESUME, USER_PROFILE, AI_RECOMMENDATION, JOB_TARGETING, PROJECT, MANUAL_ENTRY.

USER_APPROVED means the candidate explicitly accepted an AI recommendation for resume inclusion.

---

## 13. JOB SOURCES

```go
type JobSource interface {
    Name() string
    Fetch(ctx context.Context, cursor *Cursor) ([]RawJob, *Cursor, error)
}
```

Initial connectors: Greenhouse, Lever, Ashby. Also support: manual JD paste, manual job URL, manually created opportunity.

Do not make unauthorized scraping the foundation of the MVP. Keep connectors modular so additional sources can be added later.

---

## 14. JOB MODEL

Canonical Job: id, source, external_id, company_name, company_domain, title, normalized_title, seniority, description, country, state, city, location_text, remote_type, employment_type, salary_min, salary_max, salary_currency, apply_url, source_url, posted_at, first_seen_at, updated_at, last_seen_at, content_hash, status, created_at.

---

## 15. JOB DEDUPLICATION

Primary identity: source + external_id.

Secondary fingerprint: normalized company + normalized title + location + description hash.

Job ingestion must be idempotent. Do not create multiple identical jobs because a source was polled repeatedly.

---

## 16. JOB FRESHNESS

Support filters: Last 1 hour, Last 3 hours, Last 6 hours, Last 12 hours, Last 24 hours, Last 3 days, Last 7 days.

Display: 18m ago, 1h ago, 3h ago, Yesterday.

Distinguish "Posted by employer" from "First discovered by ApplyForge". Never fabricate exact posting time.

---

## 17. JOB DESCRIPTION AI EXTRACTION

JobRequirements fields: role_family, normalized_title, seniority, required_skills[], preferred_skills[], required_experience_years, responsibilities[], domains[], education_requirements[], certifications[], location_requirements, employment_type, salary, clearance_requirements, work_authorization_requirements, keywords[].

Each requirement contains: normalized_name, original_text, importance, category, confidence.

Use structured AI output. Validate with Pydantic. Never persist malformed AI responses.

---

## 18. SKILL NORMALIZATION

skill_aliases table examples: Golang → Go, K8s → Kubernetes, Postgres → PostgreSQL, AWS SQS → Amazon SQS, AWS SNS → Amazon SNS, K8 → Kubernetes.

Normalization should be deterministic where possible. AI may help identify aliases, but canonical mapping belongs in application data.

---

## 19. HARD ELIGIBILITY FILTERS

EligibilityResult: eligible, hard_failures[], warnings[].

Potential hard failures: required citizenship, required clearance, incompatible employment type, unacceptable location, explicit sponsorship incompatibility, mandatory seniority mismatch when extreme.

Ambiguous requirements should become warnings, not automatic failures.

---

## 20. JOB MATCH SCORE

Never ask an LLM "Give this resume a score from 0 to 100." Use deterministic scoring.

Recommended weights: Must-have skill coverage 30, Responsibility alignment 20, Role/seniority 15, Preferred skills 10, Domain alignment 10, Location/work arrangement 5, Education/certifications 5, Candidate preferences 5. TOTAL 100.

Return: total_score, grade, component_scores, matched_skills, transferable_skills, missing_required_skills, missing_preferred_skills, matched_responsibilities, positive_evidence, concerns, explanation.

Grades: 95–100 Exceptional, 90–94 Excellent, 80–89 Strong, 70–79 Possible, 60–69 Weak, <60 Poor. Thresholds must be configurable.

---

## 21. OPPORTUNITY SCORE

Ranking should not be identical to Job Match Score.

OpportunityScore: 75% Match, 15% Freshness, 10% Candidate Preferences.

Eligibility hard failure must override opportunity score.

---

## 22. CURRENT VS TARGET PROFILE MATCH

Calculate CURRENT PROFILE MATCH (confirmed/master resume profile only) and TARGET PROFILE MATCH (includes user-approved learning/target skills).

Never present Target Profile Match as independently verified current capability.

Example: Current Profile Match 78%, Target Profile Match 93%. Suggested additions: Amazon SQS, gRPC, DynamoDB.

---

## 23. RESUME ALIGNMENT SCORE

Separate this from Job Match Score. Resume Alignment answers: "How closely does this particular resume reflect the requirements identified in this job?"

Do not call it "ATS Score". Do not claim "93% probability of passing ATS." Call it Resume Alignment Score.

Components: required keyword coverage, preferred keyword coverage, relevant experience visibility, responsibility coverage, technical-stack alignment, role/domain alignment.

Example: Original Resume 74%, Tailored Resume 92%.

---

## 24. TRANSFERABLE SKILL ENGINE

Determine conceptual distance between existing and requested technologies.

Examples: Kafka → SQS, Kafka → Pub/Sub, Kafka → RabbitMQ, REST → gRPC, PostgreSQL → MySQL, PostgreSQL → relational modeling, PostgreSQL → DynamoDB (lower transfer score), Kubernetes → OpenShift, Docker → ECS, Jenkins → GitHub Actions, Prometheus → CloudWatch Metrics, Java/Spring → Go backend development, AWS → Terraform (partial transfer only).

Never treat related technologies as identical.

Return: source_skill, target_skill, transferability_score, level (VERY_HIGH, HIGH, MEDIUM, LOW, NONE), shared_concepts[], new_concepts_required[], reason, prep_classification.

---

## 25. SKILL PREPARATION CLASSIFICATION

QUICK_PREP: candidate possesses highly transferable knowledge and likely only needs enough preparation to discuss fundamentals.

STANDARD_PREP: candidate has some related concepts but needs meaningful study.

DEEPER_GAP: technology differs significantly from current background.

Never present exact learning-time estimates as scientifically guaranteed.

UI examples: Quick Prep, Some Preparation, Deeper Gap.

---

## 26. RESUME TAILORING MODES

STRICT: prefer only existing confirmed skills and facts.

GROWTH (default): recommend adjacent technologies using transferable-skill reasoning. User approves each meaningful addition.

MAX_MATCH: optimize strongly toward JD terminology and target technologies. Clearly identify new skills, new bullets, AI-generated claims, requirements they address. User can immediately approve — do not make the user wait to complete training.

---

## 27. AI RESUME TAILORING

Input: MasterResume, CandidateProfile, CandidateSkills, JobRequirements, TailoringMode.

Output: TailoringRun with SummarySuggestion, SkillSuggestions[], ExperienceSuggestions[], ProjectSuggestions[], KeywordCoverage.

Every TailoringSuggestion: id, section, original_text, suggested_text, requirements_addressed[], skills_added[], keywords_added[], source, reason, confidence, risk_level, user_status.

Statuses: PENDING, APPROVED, EDITED, REJECTED.

---

## 28. USER APPROVAL IS FINAL

UI actions: Approve, Edit, Reject. Also: Approve All Selected.

Once approved, the suggestion becomes part of the final tailored resume immediately. The product should not require course completion, project completion, quiz completion, waiting periods — before approval.

---

## 29. AI-INTRODUCED CLAIM DISCLOSURE

If AI introduces something not present in the master resume, show "AI Suggested".

Example: Amazon SQS. Reason: Required by this role. Transferable from: Kafka + AWS. Status: AI Suggested.

Actions: Approve & Add, Learn First, Edit, Skip.

If approved, status becomes USER_APPROVED and it may be used in the tailored resume.

---

## 30. TAILORING REVIEW UI

Build a premium resume-diff experience.

Example:

ORIGINAL: Built Go microservices for telecom file processing.

SUGGESTED: Built highly concurrent Go services processing high-volume telecom workloads through Kafka-based distributed pipelines.

Why: stronger action language, highlights scale, matches distributed systems requirement, increases Kafka relevance.

Actions: Approve, Edit, Reject.

For AI-introduced technologies, show a visible badge. Example: AWS SQS — AI Suggested — Quick Prep.

---

## 31. QUICK PREP

Every recommended unfamiliar technology should support "Learn First". Opening the control should NOT navigate away from the tailoring workflow. Use drawer/modal.

Quick Prep includes: WHAT IT IS, WHY THIS JOB NEEDS IT, WHAT YOU ALREADY KNOW THAT TRANSFERS, CORE CONCEPTS, WHAT TO KNOW FOR SCREENING, COMMON INTERVIEW QUESTIONS, COMMON MISTAKES, RELATED ARCHITECTURE QUESTIONS, EXAMPLE CODE, ASK AI, MARK COMFORTABLE.

---

## 32. DEFEND THIS BULLET

Every meaningful generated resume bullet gets "Defend This Bullet".

Example bullet: "Built event-driven Go processing workflows using Amazon SQS with retry and dead-letter queue handling."

Show likely questions: Why SQS? Why not Kafka? Standard vs FIFO? How are duplicates handled? What is visibility timeout? How do retries work? What is a DLQ? How would you scale consumers? How would you monitor it? What happens during partial failure?

For every question provide: concise answer, deeper explanation, connection to candidate's known experience.

---

## 33. "MAKE ME QUALIFIED" FEATURE

On suitable jobs, show "Make Me Qualified". When clicked, analyze: candidate strengths, missing requirements, transferable knowledge, high-value gaps, low-value requirements that can be ignored, target resume improvements.

Return: Current Profile Match, Target Profile Match, Top gaps, Recommended skills, Recommended resume changes, Quick Prep modules, Interview questions, Potential project exercises.

Example: Current Match 74%, Target Match 91%. High-value gaps: gRPC, Amazon SQS, DynamoDB. Low-priority gap: Ruby. Recommended action: Focus on gRPC, SQS and DynamoDB.

---

## 34. LEARNING PLAN

Provide optional "Prepare for This Job". Endpoint: `POST /api/v1/jobs/{jobId}/learning-plan`.

Return LearningPlan: job_id, skills[], current_readiness, target_readiness, topics[], practice_questions[], projects[], architecture_questions[], estimated_effort_category.

No forced progression.

---

## 35. INTERVIEW READINESS

Separate score. Suggested components: Core language 20, Backend fundamentals 20, Required technology 25, System design/domain 15, Experience examples 10, Question preparedness 10.

Treat score as product guidance, not scientific assessment.

---

## 36. JOBS DASHBOARD

Filters: search, title, company, location, remote type, employment type, salary, source, minimum match score, posted age, skills, saved, applied.

Sorting: Best Opportunities, Newest, Highest Match, Highest Salary. Default: Best Opportunities.

---

## 37. JOB CARD

Display: company, title, location, salary if available, posted time, employment type, Job Match Score, top matched skills, transferable skill indicator, largest gap.

Buttons: View Analysis, Tailor Resume, Save, Apply.

---

## 38. JOB DETAIL PAGE

Header: Company, Role, Location, Salary, Posted time.

Primary buttons: Tailor Resume, Make Me Qualified, Apply, Save.

Sections: Job Match, Current Profile Match, Target Profile Match, Resume Alignment, Matched Skills, Transferable Skills, Missing Skills, Quick Prep Skills, Responsibilities, Why You Match, Potential Concerns, Resume Evidence, Full Job Description.

---

## 39. MASTER VS TAILORED RESUMES

Never overwrite master resume.

ResumeVersion: id, user_id, base_resume_id, job_id nullable, version_number, content_json, match_score, alignment_score, tailoring_mode, created_at.

---

## 40. PDF AND DOCX

Generate PDF and DOCX. ATS-friendly formatting: one column, selectable text, standard headings, normal fonts, no progress bars, no image-based text, no headshots, no complicated tables.

Sections: Name/contact, Summary, Skills, Experience, Projects, Education, Certifications.

Support 1–2 pages based on experience.

---

## 41. APPLICATION FLOW

When clicking Apply: 1. verify selected resume version 2. show resume download 3. record application-preparation event 4. open official apply URL 5. allow user to confirm submitted.

Do not mark application submitted before user confirmation.

---

## 42. APPLICATION PROFILE

Save reusable answers: name, phone, email, location, desired location, work authorization, sponsorship, salary expectation, notice period, LinkedIn, GitHub, portfolio, common application questions.

User can edit at any time.

---

## 43. APPLICATION TRACKER

Statuses: SAVED, READY_TO_APPLY, APPLIED, RECRUITER_SCREEN, ASSESSMENT, TECHNICAL_INTERVIEW, FINAL_INTERVIEW, OFFER, REJECTED, WITHDRAWN.

Support Kanban and Table view.

Each application: company, role, job, resume version, match score, application date, current stage, notes, next action.

---

## 44. ANALYTICS

Dashboard: Jobs discovered, 90%+ matches, Saved jobs, Tailored resumes, Applications, Recruiter screens, Assessments, Interviews, Offers.

Conversion funnel: Applications → Responses → Screens → Technical Interviews → Offers.

Also: response rate by match score, company, role family, resume version, job freshness bucket.

Do not imply statistical significance when data is tiny.

---

## 45. AI PROVIDER ARCHITECTURE

AIProvider interface methods: ParseResume, ParseJobDescription, AnalyzeTransferableSkills, SuggestResumeTailoring, GenerateQuickPrep, DefendBullet, GenerateLearningPlan, ExplainMatch, GenerateApplicationAnswer.

Provider should initially support one AI service but remain replaceable. Business logic should not directly call vendor SDK methods everywhere. Centralize provider integration.

---

## 46. STRUCTURED OUTPUT

Use structured AI responses. Never rely on parsing long prose when a schema can be used. Validate every AI response.

Store: provider, model, operation, input_tokens, output_tokens, latency, estimated_cost, status, created_at.

---

## 47. AI COST MANAGEMENT

AI cost is likely to exceed hosting cost. Optimize aggressively.

Resume parsing: once per resume version. JD parsing: once per unique job content hash. Job matching: deterministic. AI explanations: lazy. Resume tailoring: only when requested. Quick Prep: only when requested. Defend Bullet: only when requested.

Cache reusable AI results. Do not call expensive models unnecessarily.

---

## 48. BACKGROUND JOB QUEUE

Use PostgreSQL.

background_jobs: id, job_type, payload, status, attempts, max_attempts, available_at, locked_at, locked_by, last_error, created_at, completed_at.

Workers acquire work using `SELECT ... FOR UPDATE SKIP LOCKED`.

Support: retry, bounded exponential backoff, dead-letter status, idempotency, timeouts, graceful shutdown.

---

## 49. SCHEDULER

Poll job sources periodically. Default MVP: every 1 hour. Make configurable. Avoid AI analysis of every job for every user.

Process: JOB INGESTED → normalize → deduplicate → parse requirements once → cheap candidate filtering → deterministic scoring → top matches shown → AI only used for expensive personalized operations when needed.

---

## 50. DATABASE TABLES

Create at least: users, user_profiles, job_preferences, candidate_skills, resumes, resume_versions, resume_experiences, resume_facts, companies, job_sources, jobs, job_requirements, skill_aliases, transferable_skills, job_matches, saved_jobs, tailoring_runs, tailoring_suggestions, learning_plans, quick_prep_modules, applications, application_events, application_answers, background_jobs, ai_usage, audit_events.

Use UUIDs, timestamptz, foreign keys, appropriate indexes, unique constraints.

Additional tables introduced by the immigration-aware matching requirements (see below): company_aliases, employer_h1b_stats, employer_perm_stats, immigration_evidence, immigration_compatibility.

---

## 51. API

Version: `/api/v1`.

Examples: `POST /auth/...`, `GET /profile`, `PATCH /profile`, `GET /preferences`, `PATCH /preferences`, `POST /resumes`, `GET /resumes`, `GET /resumes/{id}`, `GET /jobs`, `GET /jobs/{id}`, `GET /jobs/{id}/match`, `POST /jobs/{id}/save`, `POST /jobs/{id}/tailor`, `POST /jobs/{id}/make-me-qualified`, `POST /jobs/{id}/learning-plan`, `GET /tailoring/{id}`, `PATCH /tailoring/{id}/suggestions/{suggestionId}`, `POST /tailoring/{id}/approve-all`, `GET /skills/{skill}/quick-prep`, `POST /resume-bullets/{id}/defend`, `POST /applications`, `GET /applications`, `PATCH /applications/{id}`, `GET /analytics/dashboard`.

Generate OpenAPI docs.

---

## 52. SECURITY

Treat resumes and candidate information as sensitive.

Implement: authorization for every user resource, secure authentication, secure cookies where applicable, CSRF protection where applicable, upload limits, MIME validation, signed object URLs, rate limiting, parameterized SQL, secret management, safe error messages, account deletion, resume deletion.

Never log: passwords, access tokens, full raw resumes, private application answers, object-storage credentials, AI provider keys.

---

## 53. OBSERVABILITY

Go: slog structured JSON. Python: structured JSON logs.

Request fields: request_id, endpoint, duration, status, user_id only when appropriate.

Background jobs: job_id, job_type, attempt, duration, result.

Expose `/health`, `/ready`. Prepare architecture for OpenTelemetry. Do not overbuild observability for MVP.

---

## 54. TESTING

**Go:** unit tests, handler tests, repository tests, matching engine tests, eligibility tests, skill normalization tests, job dedupe tests, job source contract tests, background worker tests.

**Python:** pytest, schema validation, resume parser tests, JD parser tests, AI mock tests, tailoring tests, quick-prep tests, document-generation tests.

**Frontend:** component tests where useful, Playwright for critical flows.

Do NOT call real AI providers during CI. Use fixtures and mocks.

---

## 55. GOLDEN MATCHING TESTS

Fixtures: candidate_go_backend.json, candidate_java_backend.json, candidate_frontend.json, job_go_kafka_aws.json, job_java_spring.json, job_react_frontend.json, job_go_sqs_dynamodb.json.

Assertions: Go/Kafka candidate should score strongly against Go/Kafka role. Java candidate should score strongly against Java/Spring role. React candidate should not score highly against Go backend role. Kafka should create some transferability toward SQS but must not equal SQS automatically. PostgreSQL should create conceptual transferability toward DynamoDB but not identical-skill credit. Small wording changes should not create unstable score swings.

---

## 56. LOCAL DEVELOPMENT

docker-compose should provide: PostgreSQL, Go API, Python worker.

Recommended commands: `make dev`, `make test`, `make lint`, `make migrate`, `make seed`, `make build`, `make docker-up`, `make docker-down`.

Frontend should run with pnpm.

---

## 57. DEVELOPMENT SEED DATA

Create fake development data only. Include: sample user, sample backend resume, sample Java resume, sample companies, sample jobs, sample matches, sample tailored resume, sample applications.

Never include real user data.

---

## 58. CI/CD

GitHub Actions.

**Go:** gofmt, go vet, go test, go build.
**Python:** ruff, typecheck, pytest.
**Frontend:** lint, typecheck, test, build.
**Docker:** build images.

Deployment only after tests succeed.

---

## 59. DEPLOYMENT

Document exact deployment for: Cloudflare frontend, Railway Go API, Railway Python worker, Neon PostgreSQL, Cloudflare R2, GitHub Actions.

Create `.env.example`. Never commit secrets.

---

## 60. ENVIRONMENT VARIABLES

Document: DATABASE_URL, WEB_BASE_URL, API_BASE_URL, AI_WORKER_URL, AI_PROVIDER, AI_MODEL, AI_API_KEY, S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, AUTH_SECRET, AUTH_PROVIDER_KEYS, LOG_LEVEL, ENVIRONMENT, and others actually required.

---

## 61. PERFORMANCE

Initial goals: API endpoints typically <300ms excluding AI operations. Job filtering should remain fast with tens of thousands of jobs. AI work must be asynchronous where appropriate. Paginate job results. Never return massive datasets in one response.

Indexes must support: posted_at, company, normalized_title, job source, location, employment type, match score retrieval.

---

## 62. FUTURE ARCHITECTURE

Document but DO NOT implement yet: Redis, SQS/Kafka, dedicated search engine, pgvector, semantic retrieval, browser extension, official application integrations, application autofill assistant, recruiter discovery, recruiter outreach, email integration, calendar integration, subscription billing, Stripe, mobile application, team accounts, multiple AI providers, large-scale distributed workers, Kubernetes.

---

## 63. FUTURE AUTO-APPLICATION

Design interfaces so future supported auto-application can exist.

Potential abstraction: ApplicationProvider with methods CanPrepare, CanAutofill, CanSubmit, PrepareApplication, SubmitApplication, GetStatus.

MVP should focus on: resume preparation, application readiness, official apply URL, tracking. Do NOT let future auto-application requirements contaminate MVP architecture.

---

## 64. CORE PRODUCT WORKFLOW

User signs up → uploads master resume → selects target roles → ApplyForge discovers jobs → user sees fresh opportunities → user opens a 92% match → sees matched and missing skills → clicks Tailor Resume → AI proposes optimized resume → AI introduces relevant target skills → user sees which additions are AI-generated → user can immediately Approve OR click Quick Prep → tailored resume alignment increases → user downloads PDF/DOCX → opens official application → confirms application → application enters tracker → user uses Defend This Bullet before interview.

This path must feel extremely fast. Optimize heavily for this workflow.

---

## 65. UX SPEED PRINCIPLE

Do not create unnecessary friction. The ideal experience: Job discovered → analysis → tailored resume → approval → download → apply, within a very short interaction. Learning tools should enhance that flow rather than blocking it.

---

## 66. IMPORTANT UX COMPONENTS

MatchScoreRing, SkillBadge, SkillGapCard, TransferabilityCard, JobCard, JobFilters, JobAgeBadge, ResumeDiff, TailoringSuggestion, AISuggestionBadge, QuickPrepDrawer, DefendBulletDrawer, ApplicationStageBadge, AnalyticsCard, ResumePreview, OpportunityScoreIndicator.

---

## 67. ERROR STATES

Handle: AI unavailable, job source unavailable, resume parsing failure, unsupported resume, malformed job description, missing application URL, document generation failure, background job failure, expired storage URL, network interruption.

Do not show raw stack traces to end users. Provide useful retry actions.

---

## 68. ARCHITECTURAL PRINCIPLES

Prefer: simple interfaces, clear package boundaries, dependency inversion around external services, domain-driven package organization where useful, explicit error handling, context propagation, idempotency, transactions, deterministic scoring, structured AI output, configuration through environment.

Do NOT create abstraction for abstraction's sake.

---

## 69. GO PACKAGE ORGANIZATION

```
apps/api/
  cmd/
    api/
    worker/
  internal/
    auth/
    users/
    profile/
    resume/
    skills/
    jobs/
    matching/
    tailoring/
    applications/
    analytics/
    scheduler/
    background/
    storage/
    ai/
    database/
    httpapi/
```

Do not force this structure if a cleaner equivalent emerges.

---

## 70. PYTHON ORGANIZATION

```
apps/ai-worker/
  app/
    main.py
    api/
    models/
    providers/
    resume/
    jobs/
    tailoring/
    learning/
    documents/
    services/
    core/
  tests/
```

Keep AI provider code isolated.

---

## 71. FRONTEND ORGANIZATION

```
apps/web/
  app/
  components/
  features/
    auth/
    onboarding/
    jobs/
    resumes/
    tailoring/
    learning/
    applications/
    analytics/
  lib/
  hooks/
  types/
```

Keep large domain logic out of generic components.

---

## IMMIGRATION-AWARE JOB MATCHING

Immigration compatibility is a first-class product requirement.

Many ApplyForge users may currently hold H-1B status and require an employer to file an H-1B change-of-employer/transfer petition.

For those users, jobs that explicitly refuse immigration support should not appear as high-quality opportunities regardless of technical match.

The product must separately analyze:

1. H-1B transfer compatibility
2. New H-1B sponsorship
3. Future employment-based permanent-residence support
4. PERM / green-card sponsorship history
5. Role-specific immigration restrictions
6. Company-level historical sponsorship activity

Do NOT collapse all of these into one boolean called "sponsorship".

### User Immigration Preferences

Extend JobPreferences with: immigration_status, requires_h1b_transfer, requires_new_h1b_cap_sponsorship, requires_future_employment_sponsorship, green_card_support_preferred, green_card_support_required, perm_support_preferred, immigration_support_min_confidence.

For a user who already holds H-1B status, H-1B transfer support must be treated separately from new H-1B cap sponsorship.

### Immigration Compatibility Model

ImmigrationCompatibility fields: job_id, company_id, h1b_transfer_status, new_h1b_sponsorship_status, green_card_status, perm_status, role_specific_status, company_history_status, overall_status, confidence, evidence[], warnings[], last_evaluated_at.

Possible statuses: CONFIRMED_SUPPORTED, LIKELY_SUPPORTED, UNCLEAR, LIKELY_NOT_SUPPORTED, EXPLICITLY_NOT_SUPPORTED, NOT_APPLICABLE.

### H-1B Transfer Status

Must specifically answer: "Does available evidence indicate this employer can support an existing H-1B holder changing employers for this role?"

Do NOT assume "no new sponsorship" means "no H-1B transfer" — some employers distinguish between the two. Likewise, do NOT assume "company historically filed H-1Bs" means "this particular job accepts H-1B transfers."

### Role-Level Immigration Language

Extract immigration-related language from: job description, application page, job application questions, company career page, role FAQ, recruiting FAQ when available.

Detect positive phrases (e.g. "visa sponsorship available", "H-1B transfers supported", "we support change of employer petitions") and negative phrases (e.g. "will not sponsor", "no visa sponsorship", "not eligible for visa sponsorship") and semantic equivalents.

Ambiguous wording such as "must currently be authorized to work" must NOT automatically be interpreted as no H-1B transfer. Analyze the entire context.

### Role-Specific Override

Role-specific immigration language has the highest priority — it overrides company history. Example: company historically filed 2,000 H-1Bs, but current job says "We will not sponsor applicants for employment visa status" → h1b_transfer_status = EXPLICITLY_NOT_SUPPORTED. The job must not receive sponsorship credit from company history.

### Transfer-Specific Exception

A JD may say it can support visa transfers but will not sponsor a new H-1B cap petition. Interpret this as h1b_transfer_status = CONFIRMED_SUPPORTED and new_h1b_sponsorship_status = EXPLICITLY_NOT_SUPPORTED. Do not reject it merely because new H-1B sponsorship is unavailable.

### Immigration Evidence

ImmigrationEvidence fields: id, job_id nullable, company_id, evidence_type, source_type, source_url, source_date, raw_text, normalized_signal, signal_strength, confidence, supports_h1b_transfer, supports_new_h1b, supports_perm, supports_green_card, contradicts_sponsorship, created_at.

Possible source_type: JOB_DESCRIPTION, APPLICATION_FORM, COMPANY_CAREERS_PAGE, COMPANY_IMMIGRATION_POLICY, DOL_LCA_DATA, DOL_PERM_DATA, PUBLIC_GOVERNMENT_DATA, TRUSTED_EXTERNAL_DATABASE, USER_CONFIRMED_RECRUITER_INFO.

### Evidence Priority

When evidence conflicts: 1. current role-specific explicit language 2. current application-form language 3. current company immigration/careers policy 4. recent role-specific recruiter/user-confirmed information 5. recent government filing history 6. historical company sponsorship data 7. third-party sponsorship databases 8. AI inference.

Explicit current evidence must override historical inference.

### DOL H-1B Data Ingestion

Implement an enrichment pipeline using publicly available U.S. Department of Labor Office of Foreign Labor Certification disclosure data (LCA disclosure data). Store aggregated employer-level sponsorship indicators.

EmployerH1BStats fields: company_id, fiscal_year, total_lca_filings, certified_lca_filings, software_related_filings, recent_filings, states[], common_job_titles[], median_wage nullable, data_source, last_updated_at.

Do not interpret an LCA filing as a guarantee that the company will sponsor every role — it represents historical/recent sponsorship evidence only.

### PERM / Green Card Data

Use U.S. DOL PERM disclosure data where available.

EmployerPermStats fields: company_id, fiscal_year, total_perm_filings, certified_perm_cases, software_related_cases, recent_perm_activity, common_job_titles[], states[], last_updated_at.

PERM history is evidence only — it does NOT guarantee the current employee will receive PERM, when it will begin, that every business unit sponsors, that every role is eligible, or that an I-140 will ultimately be filed. Clearly communicate this distinction in the UI.

### Green Card Support Status

Values: CONFIRMED_SUPPORTED, HISTORICAL_EVIDENCE, LIKELY_SUPPORTED, UNKNOWN, EXPLICITLY_NOT_SUPPORTED.

### Green Card / I-140 Product Wording

Avoid saying "This company will file your I-140" unless current authoritative evidence explicitly supports that claim. Prefer: "Green-card sponsorship history", "Recent PERM activity", "Permanent-residence support appears available", "Green-card policy unclear", "Confirm PERM/I-140 policy with recruiter". The UI should distinguish historical evidence from a confirmed company policy.

### Company Immigration Profile

Each company should have an Immigration Profile showing: H-1B Transfer status, recent H-1B activity (FY filings + trend), Green Card / recent PERM activity, software-engineering PERM history, confidence, sources, last updated.

### Job Card Immigration Badges

Job cards should prominently display immigration compatibility, e.g. "H-1B Transfer ✓ Supported", "Green Card ✓ Strong History", "H-1B Transfer ? Unclear", "H-1B Transfer ✕ No Sponsorship — NOT ELIGIBLE".

For users who require H-1B transfer, immigration incompatibility should override technical match.

### Immigration Hard Filter

For users with requires_h1b_transfer = true, explicit role-level statements such as "no sponsorship now or in the future" must result in eligible = false, reason: IMMIGRATION_INCOMPATIBLE. Do not let a 98% technical match override this.

### Unknown Sponsorship

Unknown must NOT equal No. If no explicit immigration language is found but the company has recent H-1B activity, status may become LIKELY_SUPPORTED with appropriate confidence. If no evidence exists: UNCLEAR. Do not automatically reject unless the user configured "Hide unknown sponsorship roles."

### User Filters

H-1B Transfer: Any, Confirmed, Confirmed or Likely, Exclude No-Sponsorship, Only Confirmed.

Green Card: Any, Recent PERM History, Strong PERM History, Confirmed Company Support, Exclude Explicit No-Green-Card.

Sponsorship Confidence: High, Medium+, Any.

### Default Filter For H-1B Users

If requires_h1b_transfer = true, default job-search behavior: hide EXPLICITLY_NOT_SUPPORTED, rank down LIKELY_NOT_SUPPORTED, show but warn UNCLEAR, prioritize CONFIRMED_SUPPORTED and LIKELY_SUPPORTED.

### Sponsorship Score

Do not mix immigration compatibility completely into technical JobMatchScore. Create ImmigrationScore (0–100) from signals such as: explicit transfer support (strong positive), explicit rejection (hard failure), application asks about H-1B transfer (useful signal), recent H-1B filing history (positive), recent software-role filings (additional positive), recent PERM activity (green-card positive), explicit green-card support (strong positive), old filings only (weak signal), no evidence (neutral/unknown).

Do not create false precision. The score primarily powers ranking. Always show the underlying categorical assessment.

### Immigration-Aware Opportunity Score

For an H-1B transfer user, modify OpportunityScore: Technical Match 60%, Immigration Compatibility 20%, Freshness 10%, Candidate Preferences 10%. BUT explicit H-1B incompatibility = INELIGIBLE regardless of score. Green-card support should provide an additional ranking advantage when green_card_support_preferred = true.

### Application Question Analysis

When application metadata contains questions such as "Will you now or in the future require sponsorship?", "Do you require H-1B sponsorship?", "Will you require an H-1B transfer?", "Will you require permanent residency sponsorship?", "Will you require PERM sponsorship?" — extract these as immigration evidence. Do not automatically infer rejection simply because an application asks the question.

### Company Name Normalization

Government immigration datasets may contain legal employer names that differ from consumer-facing company names (parent companies, subsidiaries, LLCs, corporations, historical names).

CompanyAlias fields: company_id, legal_name, normalized_name, alias, source, confidence.

Do not join DOL sponsorship data to companies using naive string equality alone. Use normalized matching plus reviewed aliases. False company matches would seriously damage sponsorship recommendations.

### Immigration Confidence

HIGH: current role/company explicitly addresses the issue. MEDIUM: strong recent company history but current role is silent. LOW: only weak/historical/inferred evidence.

### Explainable Immigration Results

Never display only "H1B: 83%". Instead show the categorical status, the underlying evidence bullets, and the confidence level. This makes the decision auditable.

### Recruiter Confirmation

For unclear opportunities, generate a short optional recruiter question, e.g. "Does this position support an H-1B change-of-employer petition for a candidate who already holds H-1B status?" and a separate green-card question. Do not automatically contact recruiters in MVP. Allow the user to copy the questions.

### Application Tracker Immigration Data

Extend Application with: h1b_transfer_status, green_card_support_status, immigration_notes, recruiter_confirmed_h1b, recruiter_confirmed_green_card, confirmation_date.

Users should be able to update these after recruiter conversations. User-confirmed information applies to that opportunity only and should not automatically become a universal company policy.

### Immigration Search Experience

For an H-1B user, Jobs dashboard should prominently expose: H-1B Compatible Jobs, Strong Green-Card Sponsors, Sponsorship Unclear, Do Not Apply — Sponsorship Conflict. The default primary feed should exclude explicit conflicts.

### High-Value Job Indicator

Optional "HIGH-VALUE FOR YOU" badge when: technical_match >= configurable threshold AND H-1B transfer = CONFIRMED_SUPPORTED or LIKELY_SUPPORTED AND recent posting AND optionally recent PERM history.

### Immigration Data Refresh

Job postings: hourly or configured interval. Company immigration policies: periodically / when stale. DOL sponsorship datasets: when new quarterly data becomes available. Store: source_version, retrieved_at, effective_period.

### Important Product Rule

For an H-1B transfer user, TECHNICAL FIT ALONE IS NOT A VALID OPPORTUNITY. The system should answer: 1. Am I technically qualified? 2. Does this specific role appear compatible with an H-1B transfer? 3. What evidence supports that conclusion? 4. Does the employer have recent H-1B sponsorship history? 5. Does the employer have recent PERM/green-card history? 6. Is the evidence role-specific or only company-level? 7. How confident are we? 8. Is this job worth applying to immediately?

The goal is to avoid wasting applications on roles that explicitly cannot support the user's immigration requirements while prioritizing companies and openings with strong evidence of H-1B transfer and long-term immigration support.

---

## 72. IMPLEMENTATION PHASES

Build incrementally. Do NOT generate the entire system in one uncontrolled change.

**Phase 0** — Architecture + repository scaffolding: monorepo, docs, Next.js scaffold, Go scaffold, Python scaffold, PostgreSQL, Docker Compose, Makefile, CI skeleton, health endpoints.

**Phase 1** — Database foundation, authentication, user profiles, job preferences, onboarding.

**Phase 2** — Master resume upload, storage, PDF/DOCX extraction, structured parsing, candidate skill profile, resume review UI.

**Phase 3** — Job ingestion: Greenhouse, Lever, Ashby, normalization, deduplication, freshness, scheduler.

**Phase 4** — JD parsing, JobRequirements, skill normalization, requirements storage.

**Phase 5** — Matching engine: eligibility, deterministic scoring, transferable skills, Opportunity Score, Current vs Target Match, tests.

**Phase 6** — Jobs UI: filters, job cards, job detail, match explanations.

**Phase 7** — Resume tailoring: STRICT/GROWTH/MAX_MATCH, suggestions, diff UI, approvals, Resume Alignment.

**Phase 8** — Quick Prep, Defend This Bullet, Make Me Qualified, Interview Readiness, learning plans.

**Phase 9** — Resume generation: PDF, DOCX, versioning, preview.

**Phase 10** — Applications: tracking, Kanban, application answers, events.

**Phase 11** — Analytics: conversion funnel, response rates, match-score analytics.

**Phase 12** — Production hardening: security, observability, performance, deployment, CI/CD, documentation.

---

## 73. WORKING STYLE FOR COPILOT

For EVERY phase, before writing code: 1. review existing repository 2. read relevant architecture docs 3. state assumptions 4. identify components affected 5. identify database changes 6. identify API changes 7. identify tests required.

Then implement.

After implementation: 1. run formatters 2. run lint 3. run unit tests 4. run integration tests where relevant 5. run builds 6. fix failures 7. update documentation 8. summarize files changed 9. summarize architectural decisions 10. state remaining technical debt.

Do not claim success if tests fail. Do not leave critical code as TODO. Do not silently skip requirements.

---

## 74. FIRST COPILOT TASK

Start ONLY with Phase 0.

Create: monorepo structure, PRODUCT.md, ARCHITECTURE.md, DATABASE.md, API.md, AI_PIPELINE.md, MATCHING_ENGINE.md, RESUME_TAILORING.md, JOB_INGESTION.md, SECURITY.md, DEPLOYMENT.md, IMPLEMENTATION_PLAN.md, DECISIONS.md.

Then scaffold: Next.js web, Go API, Python AI worker, PostgreSQL Docker Compose, Makefile, .env.example, GitHub Actions skeleton, Health endpoints, README.

Verify: frontend builds, Go builds, Python imports/tests work, Docker Compose starts successfully, PostgreSQL connectivity works.

Then stop. Do NOT begin Phase 1 automatically.

At the end return: 1. repository tree 2. architecture summary 3. decisions made 4. commands to run locally 5. tests/build commands executed 6. failures encountered and fixes 7. next Phase 1 plan.

---

## 75. DEFINITION OF MVP SUCCESS

MVP is successful when a user can: 1. Create an account. 2. Upload a master resume. 3. Configure target jobs. 4. See recently discovered jobs. 5. Filter to jobs posted recently. 6. See a meaningful Job Match Score. 7. Understand why they match. 8. See transferable and missing technologies. 9. Tailor their resume. 10. Approve AI-recommended skill/bullet changes immediately. 11. Use Quick Prep for unfamiliar skills. 12. Use Defend This Bullet. 13. Download PDF/DOCX. 14. Apply through the official job link. 15. Track the application. 16. See basic interview/application analytics.

Optimize this workflow before building anything else.

---

## 76. FINAL ENGINEERING DIRECTIVE

Prioritize in this order: 1. Product usefulness 2. Correctness 3. Speed of user workflow 4. Maintainability 5. AI cost efficiency 6. Reliability 7. Security 8. Developer experience 9. Future scalability.

Do not optimize for millions of users before we have users. Do not build infrastructure just because large technology companies use it.

Use Go for reliable backend orchestration. Use Python where the AI/document ecosystem provides clear advantages. Use PostgreSQL aggressively for the MVP.

Keep AI calls structured, observable, cached, and cost-controlled. Keep scoring deterministic. Keep AI recommendations explainable. Keep the user in control.

The primary product experience should always remain:

**Discover → Match → Tailor → Approve → Prepare → Apply → Track.**
