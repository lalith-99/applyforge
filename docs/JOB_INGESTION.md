# ApplyForge — Job Supply & Ingestion

Job acquisition is infrastructure, not candidate-specific matching logic. ApplyForge maintains a canonical job catalog first; candidate eligibility, H-1B preference, ranking, resume alignment, and tailoring run downstream of that catalog.

## Supply-plane architecture

```text
Company / sponsor evidence      Broad discovery feeds
          │                              │
          ├──────────────┐     ┌─────────┤
          ▼              ▼     ▼         ▼
   company identity   source discovery / ATS directories
          │                    │
          └────────────┬───────┘
                       ▼
               company_source_registry
                       │
             verify / inspect / promote
                       ▼
                  job_sources
                       │
                 poll scheduler
                       │
        ┌──────────────┼────────────────┐
        ▼              ▼                ▼
    direct ATS     career pages    broad providers
        └──────────────┼────────────────┘
                       ▼
                    RawJob
                       ▼
       normalize → classify → deduplicate
                       ▼
              canonical jobs catalog
                       ▼
       immigration / candidate enrichment
                       ▼
           eligibility → match → rank
```

The important boundary is that the catalog exists independently of a user. The H-1B sponsor watchlist prioritizes which employer-direct sources are worth discovering and how often they should be polled; it does not prevent broad discovery sources from teaching ApplyForge about employers and postings outside the existing source graph.

## Connector contract

```go
type JobSource interface {
    Name() string
    Fetch(ctx context.Context, cursor *Cursor) ([]RawJob, *Cursor, error)
}
```

Current employer-direct connectors include Greenhouse, Lever, Ashby, SmartRecruiters, Workable, Workday, iCIMS and SuccessFactors. ApplyForge can also monitor public career pages that expose structured `schema.org/JobPosting` data. Broad/gap discovery adapters include Arbeitnow, Bright Data LinkedIn keyword discovery, and optional SerpAPI Google Jobs.

iCIMS is deliberately verification-gated. Public listing pages are used to enumerate the complete active ID set, while `schema.org/JobPosting` data on detail pages supplies richer metadata and `hiringOrganization` identity. An iCIMS registry candidate becomes monitorable only after that provider-backed employer identity agrees with the sponsor/company identity. Tenant-local numeric job IDs are namespaced with the iCIMS tenant host so `(source, external_id)` remains globally idempotent across employers.

SuccessFactors is also verification-gated. ApplyForge supports credential-free Recruiting Marketing / Career Site Builder RSS feeds and legacy Recruiting Management `Job-Listing` XML feeds. Feed-provided employer identity must match the sponsor/company identity before the source is promoted to direct polling. Historical free-bootstrap entries stored as `CUSTOM` with a `SUCCESSFACTORS|...` token are re-inspected and promoted without requiring rediscovery. Feed IDs are namespaced by tenant/company identity and complete feeds participate in closure detection.

Unsupported ATS portals are still valuable: Oracle and known custom vendors such as Eightfold, Taleo, Phenom, Avature, ADP, Cornerstone, UKG and Jobvite are retained in `company_source_registry` so connector work can be prioritized from measured coverage instead of rediscovering companies later.

## Source discovery flywheel

A source should ideally be paid/discovered once and then polled directly:

```text
unknown company
    ↓
ATS inventory / observed apply URL / career-page inspection / optional web discovery
    ↓
identify careers URL + ATS tenant/site
    ↓
company_source_registry
    ↓
verify monitorability
    ↓
job_sources
    ↓
direct polling on sponsor-tier cadence
```

Known ATS URLs are detected from ingested `apply_url` and `source_url` values and promoted automatically when the company belongs to the sponsor watchlist. Optional DataForSEO discovery searches unresolved sponsor companies for their career/ATS endpoints. The public ATS directory bootstrap provides a free first pass across the sponsor watchlist.

A company may legitimately have more than one external ATS tenant because of acquisitions, business units or migrations. The free source bootstrap therefore permits a small bounded set of up to three high-confidence directly monitorable sources per company. Additional candidates remain registry-only. Cross-source canonicalization prevents duplicate postings from surfacing independently.

## Ingestion and deduplication

Primary identity is `source + external_id`. Secondary identity is a normalized cross-source fingerprint based on company, title, location and description. Repeated polling is idempotent. When a new posting matches an already-active canonical job from another source, the higher-priority source becomes canonical and the duplicate is linked instead of displayed separately.

Direct full-snapshot sources can close postings that disappear from the board. Aggregator and bounded-discovery sources are not allowed to infer closure merely because a posting did not appear in one poll. Connectors with a safety pagination bound, including iCIMS, fail the poll instead of performing closure when the complete listing cannot be proven.

## Freshness

The catalog stores both employer-provided `posted_at` and ApplyForge `first_seen_at`. Exact posting times are never fabricated. User-facing filters can use last 1h/3h/6h/12h/24h/3d/7d while acquisition health separately measures how many canonical U.S. software jobs and employers actually produced inventory in the last 24 hours.

## Scheduling and workers

The scheduler queues one `sync_job_source` task per due source instead of polling every source serially. Background workers can therefore fetch independent ATS tenants concurrently while retaining Postgres as the queue and scheduler state store for the MVP. Slow or permanently invalid boards do not block other sources; permanent source failures are quarantined.

Sponsor tiers control direct-source cadence: HOT sources are checked most frequently, then WARM, COOL and COLD. Broad providers have their own explicit budget and polling cadence. This keeps the direct-source catalog inexpensive while preserving fresh discovery.

## Acquisition control plane

`GET /api/v1/admin/job-sources/health` is the primary supply-plane diagnostic endpoint. It reports:

- catalog freshness and canonical U.S. software inventory;
- queue backlog/dead-letter state;
- latest per-source poll results and latency;
- source discovery and provider usage;
- sponsor coverage by HOT/WARM/COOL/COLD tier;
- source-registry, monitorable-source and enabled-direct-source company counts;
- quarantined and stale enabled sources;
- fresh job-producing companies, rather than only raw source/job counts;
- coverage by source type, including registered, monitorable, enabled, failing and fresh-producing counts.

The key operating funnel is:

```text
sponsor companies
  → companies with discovered sources
  → companies with monitorable sources
  → companies with enabled direct sources
  → companies polled successfully
  → companies producing fresh jobs
  → companies producing fresh U.S. software jobs
```

Connector/source work should target the stage with the largest measured drop instead of adding providers blindly.

## Canonical Job model

The canonical catalog includes source identity, employer identity, title/classification, description, normalized location/workplace metadata, employment type, compensation where available, apply/source URLs, employer posting time, ApplyForge discovery/update timestamps, content hash, cross-source fingerprint/canonical link and lifecycle status.

## Local development bootstrap

Non-production startup performs a best-effort free sponsor-source bootstrap using public ATS inventories and the H-1B sponsor watchlist. Verified supported ATS sources are promoted to `job_sources`; unsupported or lower-confidence portals remain in `company_source_registry` for later inspection/connectors. Structured career-page inspection and paid discovery providers remain explicit environment-controlled capabilities.

No hardcoded seed-company list is the production acquisition strategy. The intended steady state is an independently maintained source graph plus broad discovery feeds that continuously teach the graph about new employers, ATS tenants and postings.
