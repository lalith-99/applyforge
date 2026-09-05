# ApplyForge — Deployment

## Target topology

| Component        | Platform            |
|-------------------|---------------------|
| `apps/web`         | Cloudflare (Pages)   |
| `apps/api`          | Railway              |
| `apps/ai-worker`    | Railway              |
| PostgreSQL          | Neon                 |
| Object storage      | Cloudflare R2        |
| Source control      | GitHub               |
| CI/CD                | GitHub Actions       |

## Local development

```
make docker-up     # Postgres + api + ai-worker via docker-compose
make migrate        # run goose migrations (Phase 1+)
make seed            # load fake dev data (Phase 1+)
cd apps/web && pnpm install && pnpm dev
```

See the root [README.md](../README.md) for the full command list and prerequisites.

## Environment variables

See [.env.example](../.env.example) at the repo root for the authoritative list. Never commit real secrets;
`.env` is git-ignored.

## Deploy sequencing (future phases)

Deployment configuration for Railway/Cloudflare/Neon/R2 will be added under `infra/` as each becomes
necessary (starting once Phase 1 introduces real, stateful functionality worth deploying). Phase 0 only
proves the stack builds and runs locally via Docker Compose.

## Phase 0 status

`infra/railway/`, `infra/cloudflare/`, and `infra/docker/` exist as placeholders with README notes; no actual
deployment has been configured or performed yet.
