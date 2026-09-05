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

## Status (post-Phase-12 — real OpenAI integration live)

Implemented endpoints: `POST /v1/resumes/extract`, `POST /v1/resumes/parse`, `POST /v1/jobs/parse-requirements`,
`POST /v1/tailoring/suggest`.

`parse_resume_text`, `parse_job_requirements`, and `generate_tailoring` (`app/resume/parsing.py`,
`app/jobs/parsing.py`, `app/tailoring/heuristics.py`) remain deterministic regex/keyword heuristics with
zero external calls, zero cost, and full test coverage — these still run whenever AI is unconfigured or
fails.

As of this pass, each of those three modules also has an `_ai`-suffixed sibling
(`parse_resume_text_ai`, `parse_job_requirements_ai`, `generate_tailoring_ai`) that calls real OpenAI
(`gpt-4o-mini` by default) via `app/providers/openai_provider.py`, using structured outputs
(`client.chat.completions.parse(response_format=<PydanticModel>)`) so the response is guaranteed to
validate against the exact same Pydantic schema the heuristic path already produced. The three route
handlers try the AI path first when `OPENAI_API_KEY` is set, and transparently fall back to the heuristic
on any `AIProviderError` (missing key, network/API error, or an empty/unparseable response) — logged as a
`logger.warning`, never a 500.

Set `OPENAI_API_KEY` (and optionally `OPENAI_MODEL`) in the shell environment or an untracked `.env` file
before `docker compose up` to enable the real AI path; see `.env.example`. Never commit a real key value.

`POST /v1/resumes/extract` (raw text extraction from uploaded PDF/DOCX) has no AI involved either way — it's
pure PyMuPDF/python-docx text extraction, not a candidate for an `_ai` sibling.

No `ai_usage` cost/latency/token logging table exists yet — now a real gap worth closing since real
OpenAI calls have a real dollar cost, unlike the heuristic path.
