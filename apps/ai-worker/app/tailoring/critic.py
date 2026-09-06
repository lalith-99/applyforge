"""AI critic for generated tailoring suggestions (Phase K). The heuristic
fallback is deliberately conservative (never flags unsupported claims it
can't actually verify, never recommends regeneration) - it exists only to
keep the endpoint functional without OPENAI_API_KEY, not as a real
substitute for the model's review, since detecting an unsupported claim
requires genuinely comparing generated text against resume evidence.
"""

from __future__ import annotations

from app.providers.openai_provider import structured_completion
from app.tailoring.critic_models import CritiqueRequest, CritiqueResult

_SYSTEM_PROMPT = """You are the final ATS and human-review quality gate for resume tailoring. Review every \
suggestion against the master resume summary and skills, which are the verified evidence of what the \
candidate has done. Also check that each suggestion maps to a required skill, preferred skill, or job \
responsibility. Flag unsupported_claims for invented tools, certifications, metrics, scope, ownership, \
or outcomes; vague_bullets for generic wording that would not help an interviewer understand impact; \
missing_high_value_keywords for important requirements absent from the suggestions; repetition for \
duplicated phrases; and ATS_issues for keyword stuffing, unclear section intent, or formatting that \
would parse poorly. Exact job keywords should be used when supported, but never at the cost of truth. \
Prefer concise bullets with action + work + verified outcome. Score ats_score (0-100) for keyword and \
parseability match, technical_match_score (0-100) for genuine fit, and human_readability (0-100) for \
specific, natural writing. Set recommend_regeneration=true for any unsupported claim, any ATS issue \
that materially harms parsing, or ats_score < 80. Give short, actionable revision instructions."""


def critique_heuristic(request: CritiqueRequest) -> CritiqueResult:
    mentioned = {s.lower() for sg in request.suggestions for s in sg.skills_added}
    missing = [
        s
        for s in (request.required_skills + request.preferred_skills)
        if s.lower() not in mentioned
    ]
    return CritiqueResult(
        missing_high_value_keywords=missing[:5],
        ats_score=70,
        technical_match_score=70,
        human_readability=70,
        recommend_regeneration=False,
        feedback="Heuristic fallback: no AI review performed.",
    )


def critique_ai(request: CritiqueRequest) -> CritiqueResult:
    suggestions_text = "\n".join(
        f"- [{sg.section}] {sg.suggested_text} (skills_added: {', '.join(sg.skills_added)}, "
        f"source: {sg.source}, risk: {sg.risk_level})"
        for sg in request.suggestions
    )
    user_prompt = f"""Job title: {request.job_title}
Master resume summary: {request.master_resume_summary or 'none'}
Master resume skills (verified evidence): {', '.join(request.master_skills) or 'none'}
Required skills: {', '.join(request.required_skills) or 'none'}
Preferred skills: {', '.join(request.preferred_skills) or 'none'}
Job responsibilities: {', '.join(request.responsibilities) or 'none'}

Generated suggestions to review:
{suggestions_text or 'none'}"""

    return structured_completion(_SYSTEM_PROMPT, user_prompt, CritiqueResult)
