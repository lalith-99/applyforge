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

_SYSTEM_PROMPT = """You are a truthfulness and quality critic reviewing AI-generated resume \
tailoring suggestions before they're shown to a candidate. Compare the suggested_text of each \
suggestion against the master_resume_summary and master_skills (the ONLY verified evidence of \
what the candidate has actually done) and flag: unsupported_claims (any suggested_text that \
asserts an accomplishment, tool, or experience NOT grounded in the master resume or an honestly-\
labeled "growth area"/"transferable" framing - fabrication is the most serious issue here), \
missing_high_value_keywords (important required_skills/preferred_skills that no suggestion \
mentions at all), weak_bullets (suggested_text that's vague, generic, or could apply to any \
candidate), and repetition (the same skill/phrase repeated near-identically across multiple \
suggestions). Score ats_score (0-100, keyword/structure match for applicant tracking systems), \
technical_match_score (0-100, genuine technical fit for required_skills), and human_readability \
(0-100, how natural and specific the writing reads to a human reviewer). Set \
recommend_regeneration=true if there are ANY unsupported_claims, or if ats_score < 80. Write a \
short, actionable feedback string usable as instructions for a revision pass (e.g. "remove the \
claimed AWS certification not present in the resume; add a keyword for Kubernetes")."""


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

Generated suggestions to review:
{suggestions_text or 'none'}"""

    return structured_completion(_SYSTEM_PROMPT, user_prompt, CritiqueResult)
