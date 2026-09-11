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

Manual refresh is asynchronous. The POST validates the request, enqueues a
`refresh_sponsor_watchlist` background job, and returns `202 Accepted`
immediately instead of holding the HTTP connection open for the database rebuild:

```bash
curl -X POST \
  -H "X-ApplyForge-Admin-Token: $ADMIN_SYNC_TOKEN" \
  "http://localhost:8080/api/v1/admin/immigration/watchlist/refresh?limit=10000"
```

Expected response:

```json
{"status":"queued","requested_limit":10000}
```

Use the summary endpoint after the background job completes:

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

Migration `00056_watchlist_refresh_performance.sql` keeps that cadence synchronized
with statement-level transition-table triggers. A 10,000-company refresh therefore
updates direct source cadence in set-based statements rather than firing 10,000
per-company `UPDATE job_sources` operations.

## Direct source discovery

Broad sources such as Bright Data, Google Jobs, Indeed-like aggregators, or any future discovery
provider can expose a direct application URL. ApplyForge inspects those URLs locally and recognizes:

- Greenhouse
- Lever
- Ashby
- SmartRecruiters
- Workable
- Workday (direct public CXS monitoring)
- iCIMS (provider-identity verification before direct polling)
- SuccessFactors (public RSS / legacy XML verification before direct polling)
- Oracle Cloud Recruiting (registry only today)

Supported public ATS connectors are inserted into `job_sources` once their source/company identity
requirements are satisfied. Greenhouse, Lever, Ashby, Workable, and suitable Workday boards can be
promoted directly from high-confidence source discovery. SmartRecruiters, iCIMS, and SuccessFactors
use provider-backed employer verification before polling is enabled. This turns paid discovery into
a bootstrap mechanism rather than a permanent dependency for every job.

Example:

```text
aggregator job
  -> https://jobs.lever.co/acme/...
  -> detect LEVER / token acme
  -> company_source_registry
  -> job_sources enabled
  -> future Acme jobs polled directly
```

Registry-only ATSs remain in `company_source_registry` and are handed to the structured career-page
inspection stage, so ApplyForge can still recover public structured jobs or discover a linked
supported ATS without rediscovering the company.

## Important limitation

The current DOL importer stores employer-level certified-case counts. It does not yet persist
SOC-level software occupation counts or worker-position counts. Therefore the first watchlist is
"H-1B sponsor likelihood first", not yet "software-H-1B share first".

That is intentional for the current H-1B transfer goal. A later importer/schema extension can add
SOC/software relevance and worker-position signals without changing the watchlist/source-discovery
architecture.

## Free HOT-source bootstrap (local MVP)

For the single-user local Docker MVP, ApplyForge can bootstrap career/ATS URLs
without a paid search provider. When `FREE_HOT_SOURCE_BOOTSTRAP_ENABLED=true`
(the non-production default), a best-effort background pass reads the public
MIT-licensed `kalil0321/ats-scrapers` company inventories and considers only
`HOT` sponsors.

The bootstrap deliberately uses conservative identity rules:

- only exact normalized employer-name matches are accepted automatically;
- Greenhouse, Lever, Ashby, Workable, and non-secondary Workday boards may be
  promoted to direct polling;
- SmartRecruiters, iCIMS, and SuccessFactors candidates are retained for
  provider-backed ownership verification before direct polling is enabled;
- Oracle, Eightfold, Taleo, Phenom, Avature, ADP, Cornerstone, UKG, and Jobvite
  URLs remain registry-only candidates until dedicated connector support exists;
- historical SuccessFactors bootstrap rows stored as `CUSTOM` with a
  `SUCCESSFACTORS|...` token are re-inspected and can be promoted without
  rediscovering the employer;
- obvious secondary Workday sites such as internal, campus, student, contractor,
  redeployment, APAC/India, or parenthesized subsidiary boards are not treated
  as the employer's primary feed;
- a small set of large first-party employers (for example Amazon, Apple, Google,
  Netflix, Salesforce, Starbucks, Tesla, and Walmart) use their official career
  search page as a registry fallback.

This pass runs asynchronously and never blocks API startup. Existing source
knowledge is refreshed at most weekly by default. After it enables a direct
source, ApplyForge immediately enqueues a source sync so newly discovered jobs
do not have to wait for the next hourly scheduler tick.

The upstream project is MIT licensed. Its directory is a discovery accelerator,
not authoritative employment/immigration evidence; ApplyForge still keeps
confidence and monitorability separate and does not infer job-level sponsorship
from the source URL.

A useful local export after bootstrap is:

```sql
SELECT
    w.watchlist_rank,
    c.name AS sponsor_name,
    csr.source_type,
    csr.board_token,
    csr.source_url,
    csr.confidence,
    csr.monitorable,
    csr.last_verified_at
FROM company_sponsor_watchlist w
JOIN companies c ON c.id = w.company_id
LEFT JOIN company_source_registry csr ON csr.company_id = w.company_id
WHERE w.tier = 'HOT'
ORDER BY w.watchlist_rank, csr.monitorable DESC, csr.confidence DESC;
```

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
`<legal employer name> careers jobs`. It inspects the organic result URLs locally and records
known ATS/career endpoints in the registry. Greenhouse, Lever, Ashby, Workable, and exact suitable
Workday endpoints can become direct sources immediately; SmartRecruiters, iCIMS, and SuccessFactors
are verification-gated and become direct sources only after provider-backed employer identity
matches the canonical company. Oracle and generic company career pages remain registry candidates
for structured inspection or future connector support rather than being guessed into unsupported
vendor APIs.

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


### Workday direct monitoring

When a discovered URL contains an exact Workday tenant and external career site, ApplyForge
promotes it into an authoritative `WORKDAY` source. The connector enumerates the complete public
CXS listing in 20-record pages and hydrates recent postings through Workday's public detail
endpoint.

To avoid thousands of detail requests for old jobs, only recent postings are hydrated by default
(`WORKDAY_DETAIL_MAX_AGE_DAYS=7`). The connector still records the complete set of external IDs,
so already-known older rows are touched before closure detection and are not falsely closed.


## Structured career-page inspection

Registry-only sources can be inspected with the optional
`CAREER_PAGE_INSPECTION_ENABLED=true` worker. It makes one bounded request to the already-discovered
career URL and looks for two deterministic signals:

1. links or embeds that resolve to a supported direct ATS;
2. schema.org `JobPosting` JSON-LD rendered in the page HTML.

Provider-specific verification is used for verification-gated ATS families such as SmartRecruiters,
iCIMS, and SuccessFactors. If another supported ATS is found, the normal direct connector is
promoted. If structured JobPosting records are found, the page is promoted to a lower-priority
`CAREER_PAGE` source and polled at the same sponsor-tier cadence.

```text
company_source_registry
  -> provider identity verifier? ------> verified direct ATS source
  -> bounded public HTML fetch
  -> ATS links? -----------------------> direct ATS source
  -> JobPosting JSON-LD? --------------> CAREER_PAGE source
  -> neither --------------------------> long retry window
```

The generic career-page source deliberately does not close jobs merely because they disappear from
one HTML page. Many career sites paginate or lazy-load results, so absence from a single page is not
strong enough closure evidence.

### Network safety

Career-page and verification requests use the existing public-internet-safe HTTP path:

- only HTTP/HTTPS URLs are accepted;
- URLs containing credentials are rejected;
- loopback, RFC1918/private, link-local, metadata, carrier-grade NAT, documentation, multicast,
  and reserved IP ranges are blocked;
- DNS is resolved before dialing and only public addresses are used;
- redirects are capped and revalidated;
- response bodies are bounded by connector-specific safety limits;
- blocked pages are not bypassed with browser automation or anti-bot evasion.

Structured generic career-page inspection is disabled by default because it performs outbound
requests to employer career sites. The default enabled configuration is intentionally modest: 100
registry entries every 30 minutes. Provider-specific verification uses the same source-inspection
queue and sponsor-priority ordering.

### Operational visibility

`GET /api/v1/admin/job-sources/health` includes a `discovery` section with watchlist status,
registry monitorability, inspection status, monthly provider-request counts, estimated provider
cost metadata, and registry coverage by source type. Direct-source coverage metrics include
verified iCIMS and SuccessFactors sources alongside the other employer ATS connectors.
