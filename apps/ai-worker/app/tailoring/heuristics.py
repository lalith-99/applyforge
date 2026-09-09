"""Heuristic (non-LLM) resume tailoring suggestion generation.

Deterministic stand-in for the AI-driven tailoring described in
MASTER_REQUIREMENTS.md §26-§27, for the same reasons as the resume/JD
parsers: a functional end-to-end approve/edit/reject pipeline without an
AI_API_KEY configured. STRICT/GROWTH/MAX_MATCH mode rules are enforced here.
"""

from __future__ import annotations

import re

from app.core.skills_dictionary import canonical_skills
from app.tailoring.models import (
    ExperienceInput,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
    TransferableMatchInput,
)


def _skill_key(value: str) -> str:
    key = value.strip().lower()
    aliases = {
        "react.js": "react",
        "reactjs": "react",
        "node.js": "nodejs",
        "node js": "nodejs",
        "golang": "go",
        "postgres": "postgresql",
        "amazon web services": "aws",
    }
    if key in aliases:
        return aliases[key]

    # Treat common language/framework version labels as the same underlying
    # skill for coverage purposes (e.g. Java 21 satisfies "Java").
    for prefix in ("java", "spring boot", "angular", "python"):
        if key.startswith(prefix + " "):
            suffix = key[len(prefix) + 1 :]
            if suffix.replace(".", "").isdigit():
                return prefix
    return key


def _lower_set(values: list[str]) -> set[str]:
    return {_skill_key(v) for v in values}


def _missing(requirements: list[str], have: set[str]) -> list[str]:
    return [r for r in requirements if _skill_key(r) not in have]


def _transfer_for(
    skill: str, transfers: list[TransferableMatchInput]
) -> TransferableMatchInput | None:
    for t in transfers:
        if _skill_key(t.target_skill) == _skill_key(skill):
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
    skill_key = _skill_key(skill)
    for exp in experiences:
        if exp.bullets and skill_key in _lower_set(exp.detected_skills):
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

    required_matched = [s for s in request.required_skills if _skill_key(s) in have]
    preferred_matched = [s for s in request.preferred_skills if _skill_key(s) in have]

    skill_suggestions: list[TailoringSuggestion] = []
    for skill in required_missing + preferred_missing:
        importance = "required" if skill in required_missing else "preferred"
        transfer = _transfer_for(skill, request.transferable_matches)
        suggestion = _skill_suggestion(skill, importance, request.mode, transfer)
        if suggestion:
            skill_suggestions.append(suggestion)

    summary_suggestion = _summary_suggestion(
        request.master_summary, request.job_title, required_matched + preferred_matched
    )
    experience_suggestion = _experience_suggestion(
        request.experiences, request.required_skills, request.preferred_skills, request.job_title
    )
    experience_suggestions = [experience_suggestion] if experience_suggestion else []

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
        "MAX_MATCH mode: suggest adding EVERY missing required and preferred skill to the skills "
        "section so keyword coverage approaches 100%, but NEVER insert an unsupported skill into "
        "Professional Experience or the professional summary. Missing skills without direct master-"
        "resume evidence stay as skill suggestions only and must be marked as requiring user "
        "verification before approval. Professional Experience must remain evidence-only."
    ),
}


_LEARNING_PHRASES = (
    "building hands-on proficiency",
    "building proficiency",
    "currently learning",
    "learning ",
    "growth area",
    "directly transferable to",
    "directly applicable to this",
)


def _skills_in_text(text: str) -> set[str]:
    lowered = text.lower()
    found: set[str] = set()
    for skill in canonical_skills():
        pattern = r"(?<![\\w+#.-])" + re.escape(skill.lower()) + r"(?![\\w+#-])"
        if re.search(pattern, lowered):
            found.add(_skill_key(skill))
    return found


def _experience_evidence_for_bullet(
    request: TailoringRequest, original_text: str
) -> set[str] | None:
    for exp in request.experiences:
        if original_text not in exp.bullets:
            continue
        evidence = _lower_set(exp.detected_skills)
        evidence.update(_skills_in_text(original_text))
        return evidence
    return None


def _sanitize_ai_tailoring(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Reject AI rewrites that turn missing keywords into fake job experience."""
    verified_master = _lower_set(request.master_skills)
    verified_master.update(
        _skill_key(skill)
        for exp in request.experiences
        for skill in exp.detected_skills
    )

    summary = result.summary_suggestion
    if summary:
        introduced = _skills_in_text(summary.suggested_text) - verified_master
        if introduced:
            summary = None

    grounded_experience: list[TailoringSuggestion] = []
    for suggestion in result.experience_suggestions:
        if not suggestion.original_text:
            continue
        evidence = _experience_evidence_for_bullet(request, suggestion.original_text)
        if evidence is None:
            continue

        lowered = suggestion.suggested_text.lower()
        if any(phrase in lowered for phrase in _LEARNING_PHRASES):
            continue

        introduced = _skills_in_text(suggestion.suggested_text) - evidence
        if introduced:
            continue
        grounded_experience.append(suggestion)

    result.summary_suggestion = summary
    result.experience_suggestions = grounded_experience
    return result


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
        "this role - section='summary' and original_text must be the exact existing summary. In STRICT "
        "or GROWTH mode, suggest up to three high-value experience bullet rewrites; in MAX_MATCH mode, "
        "you may rewrite more existing bullets, but every Professional Experience rewrite must remain "
        "strictly evidence-based. Each rewrite must be grounded in an exact existing bullet and may "
        "mention only technologies already present in that experience's detected_skills or original "
        "bullet. For a missing skill, create section='skills', original_text=null, and list it in "
        "skills_added only when the mode allows it. Transferability is useful for explaining why a "
        "skill may be learnable, but it is NOT evidence that the candidate used the target skill in a "
        "job. Never put phrases such as 'building proficiency', 'learning', 'growth area', 'directly "
        "transferable to', or 'directly applicable to this role' inside Professional Experience. "
        "Never invent a new professional-experience bullet merely to place a missing keyword. Always populate "
        "requirements_addressed with the exact requirements or responsibilities addressed, and explain "
        "the value of every suggestion in reason. Set source='MASTER_RESUME' for evidence-only rewrites "
        "and source='AI_SUGGESTED' for additions. Compute keyword_coverage_before/after as the "
        "fraction (0.0-1.0) of required_skills+preferred_skills reflected in the resume before and "
        "after suggestions; do not inflate coverage for unsupported experience claims. Preserve the "
        "base resume's compact page budget: prefer replacing/rephrasing existing text rather than "
        "making it longer. A summary rewrite should stay within roughly 10% of the original summary "
        "word count, and an experience rewrite within roughly 20% of the original bullet. Do not add "
        "new experience bullets; rewrite existing bullets and keep each suggestion concise enough for "
        "a one-page resume when the source resume was already compact."
    )
    user = request.model_dump_json(indent=2)
    result = structured_completion(
        system,
        user,
        TailoringResponse,
        model_env_var="OPENAI_TAILORING_MODEL",
    )
    return _sanitize_ai_tailoring(request, result)
