"""Heuristic (non-LLM) resume tailoring suggestion generation.

Deterministic stand-in for the AI-driven tailoring described in
MASTER_REQUIREMENTS.md §26-§27, for the same reasons as the resume/JD
parsers: a functional end-to-end approve/edit/reject pipeline without an
AI_API_KEY configured. STRICT/GROWTH/MAX_MATCH mode rules are enforced here.
"""

from __future__ import annotations

from app.tailoring.models import (
    ExperienceInput,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
    TransferableMatchInput,
)


def _lower_set(values: list[str]) -> set[str]:
    return {v.lower() for v in values}


def _missing(requirements: list[str], have: set[str]) -> list[str]:
    return [r for r in requirements if r.lower() not in have]


def _transfer_for(
    skill: str, transfers: list[TransferableMatchInput]
) -> TransferableMatchInput | None:
    for t in transfers:
        if t.target_skill.lower() == skill.lower():
            return t
    return None


def _skill_suggestion(
    skill: str, importance: str, mode: str, transfer: TransferableMatchInput | None
) -> TailoringSuggestion | None:
    if mode == "STRICT":
        return None
    if mode == "GROWTH" and transfer is None:
        return None

    if transfer is not None:
        reason = (
            f"Required by this role. Transferable from your {transfer.source_skill} experience "
            f"({transfer.level.replace('_', ' ').title()} transferability)."
        )
        confidence = 0.75
        risk_level = "LOW"
    else:
        reason = f"{importance.title()} by this role; not currently reflected on your resume."
        confidence = 0.45
        risk_level = "MEDIUM"

    return TailoringSuggestion(
        section="skills",
        original_text=None,
        suggested_text=f"Add {skill} to your skills section",
        requirements_addressed=[skill],
        skills_added=[skill],
        keywords_added=[skill],
        source="AI_SUGGESTED",
        reason=reason,
        confidence=confidence,
        risk_level=risk_level,
    )


def _summary_suggestion(
    master_summary: str | None, job_title: str, matched_skills: list[str]
) -> TailoringSuggestion | None:
    if not master_summary:
        return None
    highlight = ", ".join(matched_skills[:3]) if matched_skills else "your core technical skills"
    suggested = (
        f"{master_summary} Targeting {job_title} roles, with direct hands-on "
        f"experience in {highlight}."
    )
    return TailoringSuggestion(
        section="summary",
        original_text=master_summary,
        suggested_text=suggested,
        requirements_addressed=matched_skills[:3],
        source="AI_SUGGESTED",
        reason=(
            "Aligns your summary with the target role and foregrounds your most "
            "relevant matched skills."
        ),
        confidence=0.7,
        risk_level="LOW",
    )


def _find_experience_by_skill(
    skill: str, experiences: list[ExperienceInput]
) -> ExperienceInput | None:
    skill_lower = skill.lower()
    for exp in experiences:
        if exp.bullets and skill_lower in _lower_set(exp.detected_skills):
            return exp
    return None


def _most_relevant_experience(
    experiences: list[ExperienceInput], relevant_skills: set[str]
) -> ExperienceInput | None:
    best: ExperienceInput | None = None
    best_overlap = -1
    for exp in experiences:
        if not exp.bullets:
            continue
        overlap = len(_lower_set(exp.detected_skills) & relevant_skills)
        if overlap > best_overlap:
            best = exp
            best_overlap = overlap
    return best or (experiences[0] if experiences else None)


def _added_skills_experience_suggestion(
    bullet: str, transfer_skills: list[str], growth_skills: list[str], job_title: str
) -> TailoringSuggestion:
    """Combine every skill newly added to the skills section that maps to
    this bullet into one honest suggestion, naming every merged skill
    rather than just the first one encountered."""
    clauses = []
    if transfer_skills:
        clauses.append(
            f"a foundation directly transferable to {', '.join(transfer_skills)} "
            f"for this {job_title} role"
        )
    if growth_skills:
        clauses.append(f"related exposure to {', '.join(growth_skills)} as a growth area")
    suggested_text = f"{bullet.rstrip('.')}, with {'; and '.join(clauses)}."

    reason_parts = []
    if transfer_skills:
        reason_parts.append(f"transferable evidence for {', '.join(transfer_skills)}")
    if growth_skills:
        reason_parts.append(
            f"{', '.join(growth_skills)} framed honestly as a growth area, not a claimed "
            "accomplishment, since it isn't yet evidenced"
        )
    reason = (
        "Connects skill(s) added to the skills section (" + "; ".join(reason_parts) + ") to this "
        "existing bullet instead of leaving them isolated in the skills list."
    )

    return TailoringSuggestion(
        section="experience",
        original_text=bullet,
        suggested_text=suggested_text,
        requirements_addressed=transfer_skills + growth_skills,
        skills_added=transfer_skills + growth_skills,
        source="AI_SUGGESTED",
        reason=reason,
        confidence=0.6 if transfer_skills and not growth_skills else 0.3,
        risk_level="HIGH" if growth_skills else "MEDIUM",
    )


def _experience_suggestion(
    experiences: list[ExperienceInput],
    required_skills: list[str],
    preferred_skills: list[str],
    job_title: str,
) -> TailoringSuggestion | None:
    relevant = _lower_set(required_skills) | _lower_set(preferred_skills)
    exp = _most_relevant_experience(experiences, relevant)
    if exp is None or not exp.bullets:
        return None

    original = exp.bullets[0]
    matched = sorted(_lower_set(exp.detected_skills) & relevant)
    if not matched:
        return None

    suggested = (
        f"{original.rstrip('.')}, directly applicable to this {job_title} "
        f"role's use of {', '.join(matched)}."
    )
    return TailoringSuggestion(
        section="experience",
        original_text=original,
        suggested_text=suggested,
        requirements_addressed=matched,
        source="MASTER_RESUME",
        reason=(
            "Stronger alignment language connecting existing experience to "
            "this job's stated requirements."
        ),
        confidence=0.8,
        risk_level="LOW",
    )


def generate_tailoring(request: TailoringRequest) -> TailoringResponse:
    have = _lower_set(request.master_skills)
    required_missing = _missing(request.required_skills, have)
    preferred_missing = _missing(request.preferred_skills, have)

    required_matched = [s for s in request.required_skills if s.lower() in have]
    preferred_matched = [s for s in request.preferred_skills if s.lower() in have]

    skill_suggestions: list[TailoringSuggestion] = []
    # Group every newly-added skill by the bullet it should be reflected
    # against, so a bullet that picks up several skills gets ONE combined,
    # honestly-worded suggestion instead of several near-duplicates.
    bullet_transfer_skills: dict[str, list[str]] = {}
    for skill in required_missing + preferred_missing:
        importance = "required" if skill in required_missing else "preferred"
        transfer = _transfer_for(skill, request.transferable_matches)
        suggestion = _skill_suggestion(skill, importance, request.mode, transfer)
        if not suggestion:
            continue
        skill_suggestions.append(suggestion)

        # A skill added to the skills section should also show up in the
        # experience section, not just sit in isolation in the skills list.
        if transfer is not None:
            exp = _find_experience_by_skill(transfer.source_skill, request.experiences)
            if exp:
                bullet_transfer_skills.setdefault(exp.bullets[0], []).append(skill)

    linked_experience_suggestions = [
        _added_skills_experience_suggestion(
            bullet, bullet_transfer_skills.get(bullet, []), [], request.job_title
        )
        for bullet in bullet_transfer_skills
    ]

    summary_suggestion = _summary_suggestion(
        request.master_summary, request.job_title, required_matched + preferred_matched
    )
    experience_suggestion = _experience_suggestion(
        request.experiences, request.required_skills, request.preferred_skills, request.job_title
    )
    experience_suggestions = (
        [experience_suggestion] if experience_suggestion else []
    ) + linked_experience_suggestions

    total_reqs = len(request.required_skills) + len(request.preferred_skills)
    before_matched = len(required_matched) + len(preferred_matched)
    after_matched = before_matched + len(skill_suggestions)

    coverage_before = before_matched / total_reqs if total_reqs else 1.0
    coverage_after = min(after_matched / total_reqs, 1.0) if total_reqs else 1.0

    return TailoringResponse(
        summary_suggestion=summary_suggestion,
        skill_suggestions=skill_suggestions,
        experience_suggestions=experience_suggestions,
        keyword_coverage_before=round(coverage_before, 3),
        keyword_coverage_after=round(coverage_after, 3),
    )


_MODE_POLICY = {
    "STRICT": (
        "STRICT mode: you may only rephrase or reorder content already present in the candidate's "
        "master resume. Never suggest adding a skill, tool, or claim the candidate has not already "
        "demonstrated in their existing summary/experience bullets."
    ),
    "GROWTH": (
        "GROWTH mode: you may suggest adding a missing required/preferred skill to the skills section "
        "ONLY when the provided transferable_matches show a credible transfer path from a skill the "
        "candidate already has to that missing skill. Do not suggest any skill without transfer support."
    ),
    "MAX_MATCH": (
        "MAX_MATCH mode: you may suggest adding any missing required/preferred skill to the skills "
        "section to maximize keyword coverage for this job, but you must be honest in the 'reason' "
        "field about whether it is backed by real transferable experience (LOW risk_level) or is a pure "
        "keyword-coverage suggestion with no direct evidence (HIGH risk_level)."
    ),
}


def generate_tailoring_ai(request: TailoringRequest) -> TailoringResponse:
    """Real LLM-backed tailoring suggestion generation. Raises
    AIProviderError (see app/providers/openai_provider.py) on any failure —
    callers must fall back to generate_tailoring() above."""
    from app.providers.openai_provider import structured_completion

    system = (
        "You are an expert resume writer and ATS optimization specialist. Tailor the candidate's "
        "resume for the target job to maximize the chance of passing automated screening and earning "
        "a human interview, without ever fabricating experience, metrics, tools, certifications, "
        "scope, or outcomes. "
        + _MODE_POLICY[request.mode]
        + " Follow these priorities: (1) preserve truth and the candidate's strongest evidence; "
        "(2) use the exact wording of supported required skills and responsibilities naturally, with "
        "common synonyms only when they are accurate; (3) make bullets specific and interview-worthy "
        "using action + work performed + outcome, but only reuse metrics or outcomes present in the "
        "master resume; (4) remove filler, generic adjectives, first-person language, and keyword "
        "stuffing. Keep suggestions concise and compatible with standard ATS parsing: plain text, "
        "clear section semantics, no tables/columns/symbol-heavy formatting. Rewrite the summary (if "
        "one exists) to foreground the candidate's most relevant verified skills and experience for "
        "this role — section='summary' and original_text must be the exact existing summary. Suggest "
        "up to three high-value experience bullet rewrites, each grounded in an exact existing bullet, "
        "and connect them to the most relevant job responsibility or requirement. For a missing skill, "
        "create section='skills', original_text=null, and list it in skills_added only when the mode "
        "allows it. A skill backed by transferable_matches may receive an honest experience rewrite. "
        "A skill with no evidence must NEVER be inserted into an experience accomplishment; it may be "
        "listed as an AI_SUGGESTED high-risk keyword for MAX_MATCH review only. Do not claim hands-on "
        "ownership, proficiency, certification, or results for unsupported skills. Always populate "
        "requirements_addressed with the exact requirements or responsibilities addressed, and explain "
        "the value of every suggestion in reason. Set source='MASTER_RESUME' for evidence-only rewrites "
        "and source='AI_SUGGESTED' for additions. Compute keyword_coverage_before/after as the "
        "fraction (0.0-1.0) of required_skills+preferred_skills reflected in the resume before and "
        "after suggestions; do not inflate coverage for unsupported experience claims."
    )
    user = request.model_dump_json(indent=2)
    return structured_completion(system, user, TailoringResponse)
