"""Candidate intelligence profile synthesis: an AI-generated pass combines
resume/skills/preferences data into one structured summary (Phase F). The
heuristic fallback is a straightforward pass-through/aggregation of the
input, since there's no meaningful deterministic substitute for genuine
synthesis (e.g. inferring domains or leadership signals from bullet text) -
it exists purely so the endpoint stays functional without an AI_API_KEY, per
the same policy as every other AI-backed endpoint in this service.
"""

from __future__ import annotations

from app.candidates.models import (
    CandidateIntelligenceProfile,
    CandidateProfileRequest,
    TransferableSkillSignal,
)
from app.providers.openai_provider import structured_completion

_SYSTEM_PROMPT = """You are a technical recruiter's research assistant. Given a candidate's \
target roles, resume skills/experience, and preferences, synthesize a structured intelligence \
profile: which skills are core (strong, repeated evidence) vs secondary (mentioned once/briefly); \
what technical domains and architecture strengths the experience demonstrates; any leadership \
signals (mentoring, leading projects, managing people); and 2-4 sentences of experience_evidence \
(concrete, specific accomplishments - not generic praise). For transferable_skills, only include \
skills NOT already in core/secondary skills that are plausibly learnable given the candidate's \
existing stack (e.g. Kafka experience suggests SQS/RabbitMQ are learnable) - mark strength as \
HIGH (near-identical to existing skill), TRANSFERABLE (clear conceptual overlap), or LEARNABLE \
(plausible but a bigger stretch). Write a concise 2-3 sentence summary capturing career narrative \
and target direction. Never invent skills or accomplishments not supported by the input."""


def synthesize_profile_heuristic(request: CandidateProfileRequest) -> CandidateIntelligenceProfile:
    core = list(dict.fromkeys(request.master_skills))
    secondary: list[str] = []
    for exp in request.experiences:
        for tech in exp.technologies:
            if tech not in core and tech not in secondary:
                secondary.append(tech)

    evidence = []
    for exp in request.experiences[:3]:
        for bullet in exp.bullets[:1]:
            evidence.append(bullet)

    summary_parts = []
    if request.seniority:
        summary_parts.append(request.seniority)
    if request.target_roles:
        summary_parts.append(f"targeting {', '.join(request.target_roles[:2])}")
    summary = " ".join(summary_parts) or (request.master_summary or "")

    return CandidateIntelligenceProfile(
        target_roles=request.target_roles,
        seniority=request.seniority,
        years_experience=request.years_experience,
        core_skills=core,
        secondary_skills=secondary,
        transferable_skills=[],
        domains=request.preferred_industries,
        architecture_strengths=[],
        leadership_signals=[],
        experience_evidence=evidence,
        summary=summary,
    )


def synthesize_profile_ai(request: CandidateProfileRequest) -> CandidateIntelligenceProfile:
    experiences_text = "\n".join(
        f"- {exp.title or 'Role'} at {exp.company or 'Company'}: "
        f"{'; '.join(exp.bullets)} (tech: {', '.join(exp.technologies)})"
        for exp in request.experiences
    )
    years_exp = request.years_experience if request.years_experience is not None else "unspecified"
    user_prompt = f"""Target roles: {', '.join(request.target_roles) or 'unspecified'}
Seniority: {request.seniority or 'unspecified'}
Years of experience: {years_exp}
Resume skills: {', '.join(request.master_skills) or 'none listed'}
Resume summary: {request.master_summary or 'none'}
Preferred industries: {', '.join(request.preferred_industries) or 'unspecified'}
Preferred technologies: {', '.join(request.preferred_technologies) or 'unspecified'}
Work authorization: {request.work_authorization or 'unspecified'}

Experience:
{experiences_text or 'none listed'}"""

    return structured_completion(_SYSTEM_PROMPT, user_prompt, CandidateIntelligenceProfile)


__all__ = ["synthesize_profile_heuristic", "synthesize_profile_ai", "TransferableSkillSignal"]
