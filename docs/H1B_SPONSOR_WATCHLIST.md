# H-1B Sponsor Watchlist and Direct Source Discovery

## Goal

ApplyForge treats recent certified H-1B LCA activity as the primary company-discovery signal.
PERM is retained as a secondary green-card signal and does not affect the H-1B watchlist rank.

The default watchlist contains up to 10,000 employers ranked from the most recent three fiscal
years of imported DOL H-1B evidence.

## Watchlist refresh

Migration `00052_h1b_sponsor_watchlist.sql` creates the watchlist and immediately builds it
when DOL evidence already exists. The DOL importer also refreshes it automatically after a
successful LCA import.

Manual refresh:

```bash
curl -X POST \
  -H "X-ApplyForge-Admin-Token: $ADMIN_SYNC_TOKEN" \
  "http://localhost:8080/api/v1/admin/immigration/watchlist/refresh?limit=10000"
```

Summary:

```bash
curl \
  -H "X-ApplyForge-Admin-Token: $ADMIN_SYNC_TOKEN" \
  "http://localhost:8080/api/v1/admin/immigration/watchlist/summary"
```

Do not paste the admin token into source control.

## Ranking

Only recent certified H-1B LCA activity contributes to `priority_score`.

- current fiscal year: strongest weight
- previous fiscal year: medium weight
- two fiscal years ago: lower weight
- activity in multiple recent years: consistency bonus
- recent PERM certifications: stored separately, zero weight in H-1B rank

The refresh function first selects only the latest imported release for each
`(employer, program, fiscal_year)`, preventing cumulative quarterly DOL releases from being
double-counted.

Default cadence by rank:

| Tier | Rank | Poll interval |
| --- | ---: | ---: |
| HOT | 1-500 | 60 minutes |
| WARM | 501-2500 | 240 minutes |
| COOL | 2501-5500 | 480 minutes |
| COLD | 5501-10000 | 1440 minutes |

These intervals are copied onto direct ATS sources when they are discovered.

## Direct source discovery

Broad sources such as Bright Data, Google Jobs, Indeed-like aggregators, or any future discovery
provider can expose a direct application URL. ApplyForge inspects those URLs locally and recognizes:

- Greenhouse
- Lever
- Ashby
- SmartRecruiters
- Workable
- Workday (registry only today)
- iCIMS (registry only today)
- Oracle Cloud Recruiting (registry only today)

Supported public ATS connectors are automatically inserted into `job_sources` and enabled. This
turns paid discovery into a bootstrap mechanism rather than a permanent dependency for every job.

Example:

```text
aggregator job
  -> https://jobs.lever.co/acme/...
  -> detect LEVER / token acme
  -> company_source_registry
  -> job_sources enabled
  -> future Acme jobs polled directly
```

Unsupported ATSs remain in `company_source_registry` so future connectors or a career-page crawler
can resolve them without rediscovering the company.

## Important limitation

The current DOL importer stores employer-level certified-case counts. It does not yet persist
SOC-level software occupation counts or worker-position counts. Therefore the first watchlist is
"H-1B sponsor likelihood first", not yet "software-H-1B share first".

That is intentional for the current H-1B transfer goal. A later importer/schema extension can add
SOC/software relevance and worker-position signals without changing the watchlist/source-discovery
architecture.

## Proactive source resolution

The watchlist gives ApplyForge the company universe, but an employer name alone does not reveal its
career domain or ATS. Source discovery therefore has two inputs:

1. opportunistic detection from job/apply URLs already seen in the catalog;
2. proactive company-source resolution for unresolved sponsor-watchlist companies.

The resolver is provider-independent:

```text
company_sponsor_watchlist (PENDING / PARTIAL / FAILED)
  -> CompanySourceResolver
  -> careers / ATS candidates
  -> ATS detector
  -> company_source_registry
  -> direct job_sources when supported
```

The first implementation uses DataForSEO Google Organic Live search for
`<legal employer name> careers jobs`. It inspects the organic result URLs locally and promotes
known Greenhouse, Lever, Ashby, SmartRecruiters, or Workable endpoints into direct polling.
Workday, iCIMS, Oracle, and generic company career pages are retained in the source registry for
the structured career-page crawler rather than guessed into unsupported connectors.

### Budget controls

DataForSEO source resolution is disabled by default and requires API credentials in the local
environment. Every external request is reserved in `provider_usage` before the call. PostgreSQL
advisory locking serializes those reservations across API replicas, so daily/monthly request caps
are hard limits rather than best-effort counters.

Default bootstrap controls:

```text
batch size           500 companies
enqueue interval      15 minutes
daily request cap     10,000
monthly request cap   10,000
depth                 10 organic results
```

The default estimated request cost is only accounting metadata. Provider pricing can change, so
the request caps are the enforcement mechanism.

Provider usage can be inspected with:

```sql
SELECT
    provider,
    operation,
    status,
    count(*) AS requests,
    sum(estimated_cost_usd) AS estimated_cost_usd
FROM provider_usage
GROUP BY provider, operation, status
ORDER BY provider, operation, status;
```

Companies with a supported direct ATS stop needing source-resolution calls. Partial/failed
discoveries use long retry windows, and queue retries do not immediately repeat paid requests.
