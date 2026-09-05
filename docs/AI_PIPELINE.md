# ApplyForge — AI Pipeline

## Scope

The Python AI worker (`apps/ai-worker`) is the only service that talks to an AI provider. It centralizes all
vendor SDK usage behind an `AIProvider` interface (Phase 4+) so the provider can be swapped without touching
business logic:

```
ParseResume
ParseJobDescription
AnalyzeTransferableSkills
SuggestResumeTailoring
GenerateQuickPrep
DefendBullet
GenerateLearningPlan
ExplainMatch
GenerateApplicationAnswer
```

## Principles

* Structured output only (Pydantic-validated). Never parse free-form prose when a schema will do.
* Never persist a malformed AI response.
* Every AI call is logged with provider, model, operation, input/output tokens, latency, estimated cost,
  status — this becomes the `ai_usage` table (Phase 4+).
* Aggressive caching/dedup: resume parsing once per resume version, JD parsing once per unique job content
  hash, tailoring/Quick Prep/Defend Bullet only on explicit user request. Job matching itself is always
  deterministic and never delegated to an LLM (see [MATCHING_ENGINE.md](MATCHING_ENGINE.md)).

## Status (through Phase 7)

Implemented endpoints: `POST /v1/resumes/extract`, `POST /v1/resumes/parse`, `POST /v1/jobs/parse-requirements`,
`POST /v1/tailoring/suggest`. **All four currently use a deterministic, regex/keyword-based heuristic
implementation, not a real LLM** — there is no `AI_API_KEY` configured yet. This is a deliberate, documented
scope decision (see DECISIONS.md), not an oversight: each heuristic lives behind the same request/response
shape a real `AIProvider` call would use (`app/resume/parsing.py`, `app/jobs/parsing.py`,
`app/tailoring/heuristics.py`), so swapping in a real model later means changing the implementation inside
those functions, not the Go-side integration or API contracts.

No `ai_usage` logging table exists yet since there's no real provider call to log cost/latency for — add it
when a real `AIProvider` is wired in.
