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

## Next discovery layer

The watchlist gives ApplyForge the company universe, but an employer name alone does not reveal its
career domain or ATS. Today source discovery happens opportunistically from observed job/apply URLs.

The next provider-independent worker should resolve unresolved watchlist companies:

```text
company_sponsor_watchlist (PENDING)
  -> company/domain resolver
  -> careers URL
  -> ATS detector
  -> company_source_registry
  -> direct job_sources when supported
```

This worker can later use whichever web/company lookup provider performs best without coupling the
watchlist to Bright Data or any single vendor.
