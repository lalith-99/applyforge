# SuccessFactors direct connector

ApplyForge supports credential-free public SAP SuccessFactors Recruiting feeds as an employer-direct source.

Supported public feed families:

- Recruiting Marketing / Career Site Builder RSS at the tenant's public `/sitemal.xml` endpoint.
- Legacy Recruiting Management `Job-Listing` XML exposed through `career*.successfactors.com/career?company=...&career_ns=job_listing_summary&resultType=XML`.

## Promotion safety

A discovered SuccessFactors URL is registry-only until the public feed itself exposes an employer identity that matches the canonical company/sponsor identity. Directory membership or URL naming alone is not sufficient evidence.

Historical free-bootstrap candidates were stored as `CUSTOM` rows with a `SUCCESSFACTORS|...` board-token prefix. Migration `00061` requeues those rows. The inspection worker recognizes the prefix, verifies the public feed, and creates a first-class `SUCCESSFACTORS` registry/job-source row only after identity verification succeeds.

## Ingestion semantics

SuccessFactors feeds are treated as complete source snapshots. External IDs are namespaced by tenant/company identity so provider-local requisition IDs cannot collide between employers. Because a successful feed fetch represents the active feed, normal direct-source closure detection is enabled. A failed or oversized fetch aborts the poll, so it cannot falsely close unseen jobs.

Feed employer names are verification evidence only. Individual `RawJob` records do not override the configured canonical company, preventing feed display-name variants from creating duplicate company identities.

Permanent 404/410/422 source failures are eligible for the normal direct-source quarantine flow. Sponsor-tier polling cadence and supply-health metrics treat `SUCCESSFACTORS` like the other verified employer-direct ATS connectors.
