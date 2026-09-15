# Employers-list sponsor source bootstrap

ApplyForge can use the public `Deepakvutla9/employers-list` directory as a **source discovery accelerator** for companies that are already present in the DOL-backed H-1B sponsor watchlist.

## What it does

The bootstrap downloads the upstream `employers.json` file at runtime and exact-matches normalized employer names against `company_sponsor_watchlist`. For matched companies it:

- stores the supplied careers URL and LinkedIn Jobs URL in `company_external_links` with provider provenance;
- keeps headquarters, states, industry, historical approvals, fiscal year, and upstream update metadata as non-authoritative JSON metadata;
- detects direct Greenhouse, Lever, Ashby, Workable, Workday, and other already-supported ATS URLs through the existing source detector;
- records generic first-party career pages as `CUSTOM` registry candidates so the structured career-page inspector can resolve them later;
- treats DuckDuckGo `!ducky` career redirects as navigation-only links and never enables them as pollable job sources.

The existing `kalil0321/ats-scrapers` bootstrap still runs after this provider. DataForSEO remains a fallback for sponsor companies that cannot be resolved from free sources.

## Trust boundaries

The DOL import remains the authority for ApplyForge's H-1B priority score, watchlist rank, and HOT/WARM/COOL/COLD cadence. The employers-list approval counts are enrichment only and never overwrite DOL evidence.

Only exact normalized employer-name matches are accepted automatically. A third-party careers URL is not proof of current sponsorship policy, and job-level sponsorship exclusions still win during catalog filtering/ranking.

## Upstream/licensing

ApplyForge does not vendor or redistribute `employers.json`; it fetches the public upstream file at runtime. The upstream repository did not expose an explicit license file when this integration was added, so redistribution/copying of the dataset should not be introduced without confirming permission.
