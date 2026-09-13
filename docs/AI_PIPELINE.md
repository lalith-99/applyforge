# ApplyForge AI pipeline

The Python FastAPI service is the provider boundary. The Go API/workers own business decisions,
PostgreSQL persistence, orchestration and usage recording. The implementation uses OpenAI structured
outputs and embeddings when configured, with deterministic fallbacks for many text operations.

## Implemented behavior

- Configurable general, resume, tailoring, ranking and embedding model settings. Docker defaults are
  declared in `.env.example` and `docker-compose.yml`; bare Python provider defaults can differ.
- Pydantic response schemas and heuristic fallback paths for resume/JD parsing, candidate synthesis,
  ranking and tailoring-related operations.
- Job requirements cached per job ID and a parser-versioned content hash.
- Eager enrichment/embedding on selected new or changed dated U.S. postings; embeddings can be disabled.
- AI ranking in synchronous groups of 20, after deterministic matching; recommendation results are
  materialized in PostgreSQL.
- Tailoring and learning operations triggered by user requests.
- Usage metadata returned in HTTP headers and recorded in `ai_usage`, including model, tokens and
  optional estimated cost. Missing price settings yield NULL cost, not free usage.

## Remaining limitations

Materialized recommendations are not a durable cache of candidate/job AI judgments: refreshes rerank
unchanged pairs. Embedding storage does not record its input hash for compare-and-set writes. Concurrent
parsing misses can call the provider more than once. Heuristic fallback provenance is not consistently
represented in downstream results. One input/output pricing pair cannot accurately price all configured
model overrides; per-call metadata can be overwritten in a multi-call request. There is no enforced
monthly AI budget or atomic budget reservation before sending requests.

Structured output validates shape, not factual evidence. The generated interview probability score is
not an empirically calibrated chance of obtaining an interview. Resume suggestions still need evidence
review and scoped approval before they can be used in an application package.

## Upgrade contract

See [sections 5–7 of the architecture review](ARCHITECTURE_REVIEW_2026-09-13.md) for the full design and
current sourced pricing. In implementation order:

1. Record per-attempt/model usage and fallback provenance, including billed failures and cache charges.
2. Add versioned JD, embedding, profile, judgment and tailoring caches with cross-worker single-flight.
3. Enforce a budget using atomic worst-case reservations, reconciliation and deterministic degradation.
4. Gate high-volume work using canonical identity, eligibility and uncertainty before invoking AI.
5. Rerank changed/new candidate-job pairs; update freshness/diversity without another model call.
6. Evaluate cheaper tailoring on a held-out evidence set; escalate only on defined quality failures.
7. Bind approved resume bytes and answer hashes to a submission intent. AI must not mutate an approved
   package or perform uncontrolled external side effects.

Use offline provider Batch processing only for work that tolerates its completion window. The existing
20-job synchronous ranking request does not receive the provider Batch discount automatically. Keep
provider pricing and output/reasoning limits as explicit, versioned configuration rather than assuming
an inexpensive model makes an unbounded workflow inexpensive.
