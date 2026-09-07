# Railway deployment

ApplyForge deploys `apps/api` and `apps/ai-worker` as separate Railway
services from the same GitHub repository.

Do not add a new `railway.json` / `railway.toml` for these services. Railway
deprecated that Config-as-Code path for new services in 2026. Configure service
root directories and deployment settings through the current Railway project
configuration / CLI.

## API

- Root directory: `/apps/api`
- Dockerfile: auto-detected
- Health check: `/ready`
- Pre-deploy command: `/migrate`
- Public domain: yes
- Uses injected `PORT`

## AI worker

- Root directory: `/apps/ai-worker`
- Dockerfile: auto-detected
- Health check: `/ready`
- Public domain: no; prefer private networking
- Uses injected `PORT`

See [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md) for environment variables,
staging gates and CLI examples.
