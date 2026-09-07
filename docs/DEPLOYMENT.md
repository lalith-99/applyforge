# ApplyForge — Deployment

## Target topology

| Component | Platform | Exposure |
| --- | --- | --- |
| `apps/web` | Cloudflare Workers via vinext | Public |
| `apps/api` | Railway | Public API |
| `apps/ai-worker` | Railway | Private to API when possible |
| PostgreSQL + pgvector | Neon | Private/connection-string only |
| Resume/object storage | Cloudflare R2 | Private bucket |
| CI/CD | GitHub Actions | Repository automation |

Cloudflare's current full-stack Next.js path is Workers with vinext. Keep the existing
`next dev` / `next build` path until `pnpm check:vinext` and the staging Worker
are clean; the vinext setup is additive.

## Railway services

Create two services from this repository in both `staging` and `production`.

### API service

- Service name: `api`
- Repository root directory: `/apps/api`
- Builder: Dockerfile (Railway auto-detects `apps/api/Dockerfile`)
- Start command: image default (`/api`)
- Healthcheck path: `/ready`
- Pre-deploy command: `/migrate`
- Public domain: enabled
- Runtime port: use Railway's injected `PORT` (the API also accepts `API_ADDR`)
- Restart policy: on failure
- Deployment draining: configure a non-zero drain window for graceful shutdowns

The API image contains both `/api` and `/migrate`. The pre-deploy migration
must succeed before Railway switches traffic to the new release.

Required API variables include:

```text
ENVIRONMENT=production
DATABASE_URL=<Neon pooled PostgreSQL URL>
WEB_BASE_URL=https://<web-domain>
AI_WORKER_URL=http://<ai-worker-private-domain>:${{ai-worker.PORT}}
S3_ENDPOINT=<account-id>.r2.cloudflarestorage.com
S3_BUCKET=<private bucket>
S3_ACCESS_KEY=<secret>
S3_SECRET_KEY=<secret>
S3_USE_SSL=true
ADMIN_SYNC_TOKEN=<secret>
GOOGLE_CLIENT_ID=<secret when enabled>
GOOGLE_CLIENT_SECRET=<secret when enabled>
GOOGLE_REDIRECT_URL=https://<api-domain>/api/v1/auth/google/callback
REQUIRE_EMAIL_VERIFICATION=true
AUTH_EMAIL_FROM=<verified sender>
RESEND_API_KEY=<secret>
OPENAI_INPUT_USD_PER_1M_TOKENS=<current configured model price>
OPENAI_OUTPUT_USD_PER_1M_TOKENS=<current configured model price>
OPENAI_EMBEDDING_USD_PER_1M_TOKENS=<current embedding model price>
```

Provider secrets such as Bright Data are added only to the API service and are
never checked into Git.

### AI worker service

- Service name: `ai-worker`
- Repository root directory: `/apps/ai-worker`
- Builder: Dockerfile
- Healthcheck path: `/ready`
- Public domain: **not required** when the API uses Railway private networking
- Runtime port: Railway's injected `PORT`

Required variables:

```text
OPENAI_API_KEY=<secret>
OPENAI_MODEL=<configured chat model>
OPENAI_EMBEDDING_MODEL=<configured embedding model>
```

The API should address this service using Railway's private domain rather than a
public URL.

## Railway CLI setup

After creating/linking the Railway project, root directories can be set without
hard-coding project IDs in this repository:

```bash
railway environment staging

railway environment edit --environment staging \
  --service-config api source.rootDirectory /apps/api

railway environment edit --environment staging \
  --service-config ai-worker source.rootDirectory /apps/ai-worker
```

Set health checks and the API pre-deploy command in each service's Deploy
settings. Railway's older `railway.json` / `railway.toml` Config-as-Code path
is deprecated for new services, so ApplyForge does not add a new legacy config
file.

## Cloudflare Workers web

The web app keeps the normal Next.js scripts and adds a parallel vinext path:

```bash
cd apps/web
pnpm check:vinext
pnpm dev:vinext
pnpm build:vinext
pnpm deploy:cloudflare:staging
pnpm deploy:cloudflare
```

`vite.config.ts` configures vinext + the Cloudflare Vite plugin and
`wrangler.jsonc` defines `applyforge-web` plus a staging Worker name.

CI/non-interactive deployment requires:

```text
CLOUDFLARE_API_TOKEN
CLOUDFLARE_ACCOUNT_ID
STAGING_API_BASE_URL
```

`NEXT_PUBLIC_API_BASE_URL` must contain the full API prefix, for example:

```text
https://api.example.com/api/v1
```

## Staging gates

Before promoting a release:

1. Railway API pre-deploy `/migrate` succeeds.
2. API `/ready` is 2xx and confirms database reachability.
3. AI worker `/ready` is 2xx.
4. `GET /api/v1/admin/job-sources/health` shows healthy catalog, queue, AI and source metrics.
5. Trigger `POST /api/v1/admin/jobs/backfill` after metadata/fingerprint rule changes.
6. Cloudflare `pnpm check:vinext` passes.
7. Run the GitHub `Staging E2E` workflow with the staging web URL.
8. Verify a real fresh-US-software ingestion cycle and one resume-tailoring flow.
9. Verify password-reset/email-verification delivery.
10. Promote the same tested commit to production.

## Secrets

Never commit production/staging secrets. Use Railway variables, GitHub Actions
secrets, Cloudflare secrets/environment variables, Neon credentials and R2 API
credentials. Keep staging and production credentials separate.
