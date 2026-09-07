# Cloudflare Workers deployment

ApplyForge's full Next.js web app targets **Cloudflare Workers via vinext**, not
a static Pages export.

The deployment files live with the web application:

- `apps/web/vite.config.ts`
- `apps/web/wrangler.jsonc`
- `apps/web/package.json` vinext scripts

Before depending on Workers for production, run `pnpm check:vinext` and validate
the staging Worker. The normal `next dev` / `next build` path remains in place
during this compatibility period.

GitHub's `Deploy Staging Web` workflow expects:

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`
- `STAGING_API_BASE_URL`

No account IDs, API tokens, R2 credentials or production URLs belong in this
folder.
