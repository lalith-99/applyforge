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

## Phase 0 status

The AI worker currently exposes only `/health` and `/ready`. No provider integration, no endpoints under
`/parse`, `/tailor`, etc. exist yet — those land starting Phase 2 (resume parsing) and Phase 4 (JD parsing).
