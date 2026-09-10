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

_SYSTEM_PROMPT = (
    "You are the final ATS and human-writing quality gate for resume tailoring. Suggestions have "
    "two evidence classes. source=MASTER_RESUME means the rewrite must stay within verified resume "
    "evidence. source=AI_SUGGESTED with risk=HIGH is an attestation-gated draft: it may "
    "introduce a "
    "target technology not present in the master resume because the application will require the "
    "candidate to explicitly attest to substantially equivalent experience before approval. Do NOT "
    "flag that technology alone as an unsupported_claim when the suggestion is correctly marked "
    "AI_SUGGESTED/HIGH. Still flag invented numerical metrics, certifications, employers, dates, "
    "team sizes, promotions, or implausible business outcomes. Review whether each draft "
    "maps to an "
    "actual required/preferred skill or responsibility and whether the technical scenario is "
    "coherent with the original role context. Flag weak_bullets when text sounds AI-generated, "
    "generic, keyword-stuffed, or uses resume-inappropriate phrases such as 'learning', "
    "'building proficiency', 'gaining exposure', 'growth area', 'transferable to', "
    "'applicable to this role', or verification disclaimers. Candidate-attestation language "
    "belongs in UI metadata, never in the resume sentence. For every AI_SUGGESTED skills-section "
    "addition, require a companion experience suggestion whose suggested_text literally contains "
    "that skill; otherwise flag it as an ATS_issues item and recommend regeneration. Prefer concise "
    "human-written bullets "
    "with action + technical implementation + impact; reuse only metrics/outcomes already present "
    "in source material. Flag missing_high_value_keywords, repetition, "
    "and ATS_issues normally. Score ats_score (0-100) for keyword/parseability match, "
    "technical_match_score (0-100) for target-role fit, and human_readability (0-100) for natural "
    "specific writing. Set recommend_regeneration=true for materially weak/implausible wording, "
    "invented hard facts, major ATS issues, or ats_score < 80. Give short actionable feedback."
)


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
Master resume summary: {request.master_resume_summary or "none"}
Master resume skills (verified evidence): {", ".join(request.master_skills) or "none"}
Required skills: {", ".join(request.required_skills) or "none"}
Preferred skills: {", ".join(request.preferred_skills) or "none"}
Job responsibilities: {", ".join(request.responsibilities) or "none"}

Generated suggestions to review:
{suggestions_text or "none"}"""

    return structured_completion(
        _SYSTEM_PROMPT,
        user_prompt,
        CritiqueResult,
        model_env_var="OPENAI_TAILORING_MODEL",
    )
