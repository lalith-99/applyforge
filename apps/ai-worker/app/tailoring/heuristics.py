"""Heuristic (non-LLM) resume tailoring suggestion generation.

Deterministic stand-in for the AI-driven tailoring described in
MASTER_REQUIREMENTS.md §26-§27, for the same reasons as the resume/JD
parsers: a functional end-to-end approve/edit/reject pipeline without an
AI_API_KEY configured. STRICT/GROWTH/MAX_MATCH mode rules are enforced here.
"""

from __future__ import annotations

import json
import re

from app.core.skills_dictionary import canonical_skills
from app.tailoring.models import (
    ExperienceInput,
    ExperienceSupportResponse,
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
        "MAX_MATCH mode: aggressively optimize for the target job. Every missing skill that you "
        "propose adding to the skills section MUST also have at least one polished Professional "
        "Experience rewrite that uses that skill in a coherent, realistic technical scenario. Draft "
        "the support bullet from the most context-compatible existing experience, not a random one. "
        "These drafts "
        "are review candidates only: set source='AI_SUGGESTED', risk_level='HIGH', and include every "
        "new technology in skills_added so the application can require explicit candidate attestation "
        "before the bullet is allowed into a final resume. Do not weaken the resume sentence with "
        "learning, hypothetical, transferable, or verification language."
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
        pattern = r"(?<![\w+#.-])" + re.escape(skill.lower()) + r"(?![\w+#-])"
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


def _display_names_for_keys(request: TailoringRequest, keys: set[str]) -> list[str]:
    candidates = (
        request.required_skills
        + request.preferred_skills
        + request.master_skills
        + canonical_skills()
    )
    out: list[str] = []
    seen: set[str] = set()
    for value in candidates:
        key = _skill_key(value)
        if key in keys and key not in seen:
            out.append(value)
            seen.add(key)
    return out


def _text_mentions_skill(text: str, skill: str) -> bool:
    value = skill.strip().lower()
    if not value:
        return False
    pattern = r"(?<![\w+#.-])" + re.escape(value) + r"(?![\w+#-])"
    return bool(re.search(pattern, text.lower()))


def _skill_keys_in_text(request: TailoringRequest, text: str) -> set[str]:
    found = _skills_in_text(text)
    for skill in request.required_skills + request.preferred_skills + request.master_skills:
        if _text_mentions_skill(text, skill):
            found.add(_skill_key(skill))
    return found


def _experience_support_keys(result: TailoringResponse) -> set[str]:
    candidate_skills = [
        skill
        for suggestion in result.skill_suggestions
        for skill in suggestion.skills_added
        if skill.strip()
    ]
    supported: set[str] = set()
    for suggestion in result.experience_suggestions:
        explicit = _lower_set(suggestion.skills_added)
        for skill in candidate_skills:
            key = _skill_key(skill)
            if key in explicit and _text_mentions_skill(suggestion.suggested_text, skill):
                supported.add(key)
                continue
            if _text_mentions_skill(suggestion.suggested_text, skill):
                supported.add(key)
    return supported


def _skills_missing_experience_support(result: TailoringResponse) -> list[str]:
    supported = _experience_support_keys(result)
    missing: list[str] = []
    seen: set[str] = set()
    for suggestion in result.skill_suggestions:
        for skill in suggestion.skills_added:
            key = _skill_key(skill)
            if key and key not in supported and key not in seen:
                missing.append(skill)
                seen.add(key)
    return missing


def _recompute_keyword_coverage(
    request: TailoringRequest, result: TailoringResponse
) -> None:
    before = _lower_set(request.master_skills)
    after = set(before)
    for suggestion in result.skill_suggestions:
        after.update(_lower_set(suggestion.skills_added))

    requirements = request.required_skills + request.preferred_skills
    if not requirements:
        result.keyword_coverage_before = 1.0
        result.keyword_coverage_after = 1.0
        return

    result.keyword_coverage_before = round(
        sum(1 for skill in requirements if _skill_key(skill) in before) / len(requirements),
        3,
    )
    result.keyword_coverage_after = round(
        sum(1 for skill in requirements if _skill_key(skill) in after) / len(requirements),
        3,
    )


def _drop_skills_without_experience_support(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Never ship a MAX_MATCH skill into the resume without a supporting draft."""
    supported = _experience_support_keys(result)
    filtered: list[TailoringSuggestion] = []
    for suggestion in result.skill_suggestions:
        keys = {_skill_key(skill) for skill in suggestion.skills_added if skill.strip()}
        if keys and keys.issubset(supported):
            filtered.append(suggestion)
    result.skill_suggestions = filtered
    _recompute_keyword_coverage(request, result)
    return result


def _generate_missing_experience_support_ai(
    request: TailoringRequest,
    result: TailoringResponse,
    missing_skills: list[str],
) -> ExperienceSupportResponse:
    from app.providers.openai_provider import structured_completion

    used_originals = [
        suggestion.original_text
        for suggestion in result.experience_suggestions
        if suggestion.original_text
    ]
    payload = {
        "job_title": request.job_title,
        "missing_skills_requiring_support": missing_skills,
        "required_skills": request.required_skills,
        "preferred_skills": request.preferred_skills,
        "job_responsibilities": request.responsibilities,
        "master_experiences": [experience.model_dump() for experience in request.experiences],
        "already_used_original_bullets": used_originals,
    }
    system = (
        "You are a senior technical resume writer performing a focused repair pass. "
        "The first tailoring pass added target-job skills but failed to create professional "
        "experience bullets supporting them. Produce only high-quality experience rewrites that "
        "close that gap. Every suggestion must replace one exact existing bullet from "
        "master_experiences: original_text must match the source bullet character-for-character. "
        "Choose the most context-compatible bullet based on the actual work, system type, domain, "
        "and surrounding stack - never just the first bullet. Do not reuse a bullet listed in "
        "already_used_original_bullets, and do not use the same original_text twice. "
        "The new skill must perform a concrete technical role in the sentence (implementation, "
        "integration, testing, deployment, data flow, client development, or service development), "
        "not appear as a detached keyword. Preserve the source role's domain and plausible scope. "
        "If two missing skills naturally belong to one technical scenario, cover them in one rewrite; "
        "otherwise use separate source bullets. Set section='experience', source='AI_SUGGESTED', "
        "risk_level='HIGH', and skills_added/keywords_added to the exact missing skills introduced. "
        "Every skill in skills_added must appear literally in suggested_text. Keep bullets concise, "
        "specific, and human-written (normally 18-32 words). Never use wording such as learning, "
        "building proficiency, gaining exposure, growth area, transferable to, applicable to this "
        "role, candidate verification, or similar disclaimers. Do not invent certifications, employers, "
        "dates, promotions, team sizes, numerical metrics, or named outcomes. You may preserve an "
        "existing metric only when the rewritten bullet clearly describes the same underlying outcome. "
        "If a skill cannot be integrated into any existing experience without producing an implausible "
        "or random bullet, omit that skill entirely; the application will then remove it from the "
        "skills section. Quality and coherence are more important than forcing every keyword."
    )
    return structured_completion(
        system,
        json.dumps(payload, indent=2),
        ExperienceSupportResponse,
        model_env_var="OPENAI_TAILORING_MODEL",
    )


def _sanitize_ai_tailoring(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Classify unsupported drafts for attestation without weakening resume prose.

    Verified rewrites remain MASTER_RESUME suggestions. If an experience rewrite
    introduces a technology that is not evidenced by the matched source bullet,
    preserve the strong draft but mark it AI_SUGGESTED/HIGH and enumerate the
    introduced technologies in skills_added. The Go/API layer uses that metadata
    to require explicit candidate attestation before approval.
    """
    verified_master = _lower_set(request.master_skills)
    verified_master.update(
        _skill_key(skill)
        for exp in request.experiences
        for skill in exp.detected_skills
    )

    summary = result.summary_suggestion
    if summary:
        introduced = _skill_keys_in_text(request, summary.suggested_text) - verified_master
        if introduced:
            # Keep the professional summary evidence-based; speculative content
            # belongs in individually reviewable experience/skill cards.
            summary = None
        else:
            summary.source = "MASTER_RESUME"
            summary.risk_level = "LOW"

    classified_experience: list[TailoringSuggestion] = []
    used_originals: set[str] = set()
    for suggestion in result.experience_suggestions:
        if not suggestion.original_text:
            continue

        if suggestion.original_text in used_originals:
            continue
        evidence = _experience_evidence_for_bullet(request, suggestion.original_text)
        if evidence is None:
            continue

        lowered = suggestion.suggested_text.lower()
        if any(phrase in lowered for phrase in _LEARNING_PHRASES):
            # Weak "learning/proficiency" prose should never be shown as a
            # resume bullet. The model gets one critic-driven regeneration pass.
            continue

        introduced = _skill_keys_in_text(request, suggestion.suggested_text) - evidence
        if introduced:
            introduced_names = _display_names_for_keys(request, introduced)
            suggestion.source = "AI_SUGGESTED"
            suggestion.risk_level = "HIGH"
            suggestion.skills_added = list(
                dict.fromkeys([*suggestion.skills_added, *introduced_names])
            )
            suggestion.keywords_added = list(
                dict.fromkeys([*suggestion.keywords_added, *introduced_names])
            )
            if "candidate verification" not in suggestion.reason.lower():
                suggestion.reason = (
                    suggestion.reason.rstrip(".")
                    + ". Drafted as a strong target-role bullet; candidate verification is "
                    "required before it can be approved into the final resume."
                )
        else:
            suggestion.source = "MASTER_RESUME"
            suggestion.risk_level = "LOW"
            suggestion.skills_added = []
        classified_experience.append(suggestion)
        used_originals.add(suggestion.original_text)

    result.summary_suggestion = summary
    result.experience_suggestions = classified_experience
    return result


def generate_tailoring_ai(request: TailoringRequest) -> TailoringResponse:
    """Real LLM-backed tailoring suggestion generation. Raises
    AIProviderError (see app/providers/openai_provider.py) on any failure —
    callers must fall back to generate_tailoring() above."""
    from app.providers.openai_provider import structured_completion

    system = (
        "You are an elite technical resume writer and ATS optimization specialist. Produce concise, "
        "natural, human-written, interview-worthy resume suggestions tailored to the target job. "
        "Separate writing quality from evidence classification: the application will gate unsupported "
        "AI drafts behind explicit candidate attestation before they can enter a final resume. Never "
        "invent numerical metrics, certifications, employers, dates, promotions, team sizes, or named "
        "business outcomes that are absent from the master resume. "
        + _MODE_POLICY[request.mode]
        + " Follow these priorities: (1) write like a strong human technical resume writer, not a "
        "match-analysis tool; (2) use exact job technologies naturally when appropriate; (3) make "
        "bullets concrete with action + technical implementation + engineering/business impact; "
        "(4) only reuse numerical metrics or specific outcomes already present in the master resume; "
        "(5) remove filler, generic adjectives, first-person language, and keyword stuffing. Never "
        "write resume bullets containing phrases such as 'learning', 'building proficiency', "
        "'gaining exposure', 'growth area', 'transferable to', 'applicable to this role', or "
        "'candidate verification'. Evidence/verification belongs in metadata and UI, never inside "
        "the resume sentence. Keep suggestions concise and compatible with standard ATS parsing: plain text, "
        "clear section semantics, no tables/columns/symbol-heavy formatting. Rewrite the summary (if "
        "one exists) to foreground the candidate's most relevant verified skills and experience for "
        "this role - section='summary' and original_text must be the exact existing summary. In STRICT "
        "or GROWTH mode, suggest up to three high-value evidence-based experience rewrites. In "
        "MAX_MATCH mode, also draft strong experience rewrites for important missing technologies "
        "when they can be integrated into a technically coherent scenario based on an existing bullet. "
        "Every experience rewrite must use original_text equal to an exact existing bullet so the app "
        "can replace it deterministically; do not create free-floating new employment rows. When a "
        "rewrite introduces a technology not evidenced in that source experience, set "
        "source='AI_SUGGESTED', risk_level='HIGH', and put the technology in skills_added. Write the "
        "bullet itself as a clean completed-work accomplishment because the UI, not the resume text, "
        "will carry the candidate-attestation warning. Do not invent metrics for such drafts. For a "
        "missing skill, also create section='skills', original_text=null, when the mode allows it. "
        "In MAX_MATCH, never return a skills-only addition: every skill_suggestion must be paired "
        "with an experience_suggestion that literally contains that skill in the resume sentence. "
        "Prefer one coherent technical scenario over appending a keyword to an unrelated sentence. "
        "When the JD only names a broad platform such as Azure or AWS, do not invent extra named "
        "sub-services unless the JD or source resume mentions them; use realistic platform-level "
        "implementation language instead. Keep most bullets around 18-32 words. Example quality bar: "
        "BAD: '...while building proficiency in Azure.' GOOD AI_SUGGESTED/HIGH draft: 'Deployed "
        "Spring Boot microservices on Azure using containerized CI/CD workflows, improving deployment "
        "consistency across application environments.' The GOOD sentence is resume-ready prose, while "
        "its AI_SUGGESTED/HIGH metadata tells the application to require candidate attestation. "
        "Always populate "
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
    result = _sanitize_ai_tailoring(request, result)

    if request.mode == "MAX_MATCH":
        from app.providers.openai_provider import AIProviderError

        missing_support = _skills_missing_experience_support(result)
        if missing_support:
            try:
                repair = _generate_missing_experience_support_ai(
                    request,
                    result,
                    missing_support,
                )
                result.experience_suggestions.extend(repair.experience_suggestions)
                result = _sanitize_ai_tailoring(request, result)
            except AIProviderError:
                # A skill without an experience support draft must not leak into
                # the generated resume just because the repair call failed.
                pass

        result = _drop_skills_without_experience_support(request, result)

    return result
