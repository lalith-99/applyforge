# ApplyForge — Resume Tailoring

## Modes

* **STRICT** — only existing confirmed skills/facts.
* **GROWTH** (default) — recommends adjacent technologies via transferable-skill reasoning; user approves each
  meaningful addition.
* **MAX_MATCH** — optimizes strongly toward JD terminology/target technologies; clearly flags new skills,
  bullets, and AI-generated claims. Approval is immediate — no forced waiting/training period in any mode.

## Flow

```
MasterResume + CandidateProfile + CandidateSkills + JobRequirements + TailoringMode
  → (Python AI worker) → TailoringRun
      SummarySuggestion
      SkillSuggestions[]
      ExperienceSuggestions[]
      ProjectSuggestions[]
      KeywordCoverage
```

Every `TailoringSuggestion`: `id, section, original_text, suggested_text, requirements_addressed[],
skills_added[], keywords_added[], source, reason, confidence, risk_level, user_status
(PENDING|APPROVED|EDITED|REJECTED)`.

## User control

Approve / Edit / Reject / Approve All Selected. An approved suggestion becomes part of the tailored resume
immediately — never gated on course/quiz/project completion. AI-introduced claims not present on the master
resume are always labeled "AI Suggested" with actions Approve & Add / Learn First / Edit / Skip; approving
flips the underlying `CandidateSkill.status` to `USER_APPROVED`.

## Never overwrite the master resume

Tailoring always produces a new `ResumeVersion` (`id, user_id, base_resume_id, job_id nullable,
version_number, content_json, match_score, alignment_score, tailoring_mode, created_at`). The master resume
is immutable except for explicit user edits/deletion.

## Supporting features

* **Quick Prep** — drawer/modal (never navigates away) covering what-it-is, why-needed, transferable
  knowledge, core concepts, screening expectations, interview questions, common mistakes, architecture
  questions, example code, ask-AI, mark-comfortable.
* **Defend This Bullet** — per-bullet likely interview questions with concise answer + deeper explanation +
  link back to the candidate's real experience.
* **Make Me Qualified** — on-demand analysis returning current/target match, high- vs low-value gaps,
  recommended resume changes, Quick Prep modules, interview questions, and practice project ideas.

## Phase 0 status

Not implemented. Lands in Phase 7 (tailoring core) and Phase 8 (Quick Prep / Defend This Bullet / Make Me
Qualified / Interview Readiness / learning plans).
