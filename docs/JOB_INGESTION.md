# ApplyForge — Job Ingestion

## Sources

```go
type JobSource interface {
    Name() string
    Fetch(ctx context.Context, cursor *Cursor) ([]RawJob, *Cursor, error)
}
```

Initial connectors (Phase 3): Greenhouse, Lever, Ashby. Also supported: manual JD paste, manual job URL,
manually created opportunity. No unauthorized scraping as the ingestion foundation; connectors stay modular
so more sources can be added later.

## Deduplication

Primary identity: `source + external_id`. Secondary fingerprint: normalized company + normalized title +
location + description hash. Ingestion must be idempotent — repeated polls must never create duplicate rows.

## Freshness

Filters: last 1h/3h/6h/12h/24h/3d/7d. Display as relative time ("18m ago", "1h ago", "Yesterday"). The UI
always distinguishes "posted by employer" (`posted_at`) from "first discovered by ApplyForge"
(`first_seen_at`) — exact posting time is never fabricated when unavailable.

## Scheduler

Polls job sources on a configurable interval (default: hourly). Pipeline: ingest → normalize → deduplicate →
parse requirements once (cached by content hash) → cheap deterministic candidate filtering → deterministic
scoring → show top matches. AI is only invoked for expensive, explicitly-requested, personalized operations —
never run against every job for every user automatically.

## Canonical Job model

`id, source, external_id, company_name, company_domain, title, normalized_title, seniority, description,
country, state, city, location_text, remote_type, employment_type, salary_min, salary_max, salary_currency,
apply_url, source_url, posted_at, first_seen_at, updated_at, last_seen_at, content_hash, status, created_at`.

## Phase 0 status

Not implemented. Lands in Phase 3 alongside the scheduler and the first three connectors.
