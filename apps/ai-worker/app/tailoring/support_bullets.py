# ruff: noqa: E501
"""Required supporting experience drafts for MAX_MATCH skill additions.

The primary tailoring pass and category enrichment stay authoritative. This
module is a recovery layer for the concrete failure mode where MAX_MATCH
returns many skill cards but no experience suggestions that support those
skills. It asks the tailoring model for a small number of coherent, anchored
support bullets and validates them before they reach the review UI.
"""

from __future__ import annotations

import json
import logging
from collections import Counter

from pydantic import BaseModel, Field

from app.providers.openai_provider import AIProviderError, structured_completion
from app.tailoring.enrichment import _fallback_category
from app.tailoring.models import (
    SKILL_CATEGORIES,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
)

logger = logging.getLogger(__name__)


class _OneSupport(BaseModel):
    experience_suggestions: list[TailoringSuggestion] = Field(min_length=1, max_length=1)


class _TwoSupports(BaseModel):
    experience_suggestions: list[TailoringSuggestion] = Field(min_length=2, max_length=2)


class _ThreeSupports(BaseModel):
    experience_suggestions: list[TailoringSuggestion] = Field(min_length=3, max_length=3)


def _skill_key(value: str) -> str:
    key = " ".join(str(value).strip().lower().split())
    aliases = {
        "golang": "go",
        "go (golang)": "go",
        "rag": "retrieval-augmented generation",
        "retrieval augmented generation": "retrieval-augmented generation",
        "mcp": "model context protocol",
        "mcp servers": "model context protocol",
        "model context protocol (mcp)": "model context protocol",
        "iac": "infrastructure as code",
        "infrastructure-as-code": "infrastructure as code",
        "llm based systems": "llm-based systems",
        "llm systems": "llm-based systems",
    }
    return aliases.get(key, key)


def _target_support_count(skill_count: int) -> int:
    if skill_count >= 9:
        return 3
    if skill_count >= 4:
        return 2
    return 1 if skill_count else 0


def _skill_card_map(result: TailoringResponse) -> dict[str, str]:
    out: dict[str, str] = {}
    for suggestion in result.skill_suggestions:
        for skill in suggestion.skills_added:
            out.setdefault(_skill_key(skill), skill)
    return out


def _supported_skill_keys(result: TailoringResponse, card_keys: set[str]) -> set[str]:
    supported: set[str] = set()
    for suggestion in result.experience_suggestions:
        for skill in suggestion.skills_added:
            key = _skill_key(skill)
            if key in card_keys:
                supported.add(key)
    return supported


def _supporting_bullet_count(result: TailoringResponse, card_keys: set[str]) -> int:
    count = 0
    for suggestion in result.experience_suggestions:
        if any(_skill_key(skill) in card_keys for skill in suggestion.skills_added):
            count += 1
    return count


def _exact_role_exists(
    request: TailoringRequest, company: str | None, title: str | None
) -> bool:
    if not company and not title:
        return False
    for experience in request.experiences:
        company_matches = (
            not company
            or (experience.company or "").strip().casefold() == company.strip().casefold()
        )
        title_matches = (
            not title
            or (experience.title or "").strip().casefold() == title.strip().casefold()
        )
        if company_matches and title_matches:
            return True
    return False


def _category_for_card(result: TailoringResponse, card_display: str) -> str:
    wanted_key = _skill_key(card_display)
    for suggestion in result.skill_suggestions:
        for skill, category in suggestion.skill_categories.items():
            if _skill_key(skill) == wanted_key and category in SKILL_CATEGORIES:
                return category
    return _fallback_category(card_display)


def _normalize_supported_skills(
    suggestion: TailoringSuggestion,
    card_map: dict[str, str],
    result: TailoringResponse,
) -> bool:
    """Use exact skill-card labels so the UI can link support to each card."""
    normalized: list[str] = []
    seen: set[str] = set()
    for raw_skill in suggestion.skills_added:
        key = _skill_key(raw_skill)
        display = card_map.get(key)
        if display is None or key in seen:
            continue
        normalized.append(display)
        seen.add(key)

    if not normalized:
        return False

    suggestion.skills_added = normalized
    suggestion.keywords_added = list(
        dict.fromkeys([*suggestion.keywords_added, *normalized])
    )
    suggestion.skill_categories = {
        skill: _category_for_card(result, skill) for skill in normalized
    }
    return True


def _validate_supports(
    request: TailoringRequest,
    result: TailoringResponse,
    candidates: list[TailoringSuggestion],
    limit: int,
) -> list[TailoringSuggestion]:
    source_bullets = {
        bullet for experience in request.experiences for bullet in experience.bullets
    }
    used_originals = {
        suggestion.original_text
        for suggestion in result.experience_suggestions
        if suggestion.original_text
    }
    existing_texts = {
        suggestion.suggested_text.strip().casefold()
        for suggestion in result.experience_suggestions
    }
    card_map = _skill_card_map(result)
    added_per_role: Counter[tuple[str, str]] = Counter()
    valid: list[TailoringSuggestion] = []

    for suggestion in candidates:
        if len(valid) >= limit:
            break
        if suggestion.suggested_text.strip().casefold() in existing_texts:
            continue
        if not _normalize_supported_skills(suggestion, card_map, result):
            continue

        suggestion.section = "experience"
        suggestion.source = "AI_SUGGESTED"
        suggestion.risk_level = "HIGH"

        if suggestion.operation == "ADD":
            if not _exact_role_exists(
                request, suggestion.target_company, suggestion.target_title
            ):
                continue
            role_key = (
                (suggestion.target_company or "").strip().casefold(),
                (suggestion.target_title or "").strip().casefold(),
            )
            if added_per_role[role_key] >= 1:
                continue
            suggestion.original_text = None
            added_per_role[role_key] += 1
        else:
            suggestion.operation = "REWRITE"
            if (
                not suggestion.original_text
                or suggestion.original_text not in source_bullets
                or suggestion.original_text in used_originals
            ):
                continue
            suggestion.target_company = None
            suggestion.target_title = None
            used_originals.add(suggestion.original_text)

        if "candidate verification" not in suggestion.reason.lower():
            suggestion.reason = (
                suggestion.reason.rstrip(".")
                + ". Candidate verification is required before approval."
            )
        existing_texts.add(suggestion.suggested_text.strip().casefold())
        valid.append(suggestion)

    return valid


def _response_model(count: int) -> type[BaseModel]:
    if count >= 3:
        return _ThreeSupports
    if count == 2:
        return _TwoSupports
    return _OneSupport


def _payload(
    request: TailoringRequest,
    result: TailoringResponse,
    unsupported_skills: list[str],
) -> dict[str, object]:
    return {
        "job_title": request.job_title,
        "job_description": request.job_description,
        "required_skills": request.required_skills,
        "preferred_skills": request.preferred_skills,
        "responsibilities": request.responsibilities,
        "skills_needing_support": unsupported_skills,
        "all_skill_suggestions": [
            suggestion.model_dump() for suggestion in result.skill_suggestions
        ],
        "master_experiences": [experience.model_dump() for experience in request.experiences],
        "existing_experience_suggestions": [
            suggestion.model_dump() for suggestion in result.experience_suggestions
        ],
    }


def _system_prompt(count: int) -> str:
    noun = "suggestion" if count == 1 else "suggestions"
    return (
        "You are the supporting-experience writer for MAX_MATCH technical resume tailoring. "
        f"Return exactly {count} high-quality Professional Experience {noun}. The current run "
        "already has skill cards but lacks enough experience evidence for them. Each returned "
        "suggestion MUST support one or more exact labels from skills_needing_support, and every "
        "supported label MUST be listed verbatim in skills_added so the UI can link the bullet to "
        "its skill card. Group related skills into coherent technical scenarios instead of making "
        "one keyword-stuffed bullet per skill. Good clusters include agentic AI (prompt engineering, "
        "tool calling, agent memory/orchestration/frameworks, MCP, AI observability), testing "
        "(automated frameworks, API/integration/data-pipeline testing), and infrastructure "
        "(Linux, infrastructure as code, monitoring) when those concepts are actually in the JD. "
        "Prefer operation='REWRITE': original_text must exactly equal an unused bullet from "
        "master_experiences and the rewrite should preserve that bullet's real project context while "
        "adding the target technology in a technically plausible way. Use operation='ADD' when no "
        "existing bullet can naturally carry an important skill cluster; then original_text=null and "
        "target_company/target_title must exactly match one existing master_experiences role. Do not "
        "invent employers, titles, dates, certifications, numerical metrics, team sizes, customer "
        "names, or business outcomes. Do not write learning, exposure, transferable, or verification "
        "language inside the resume sentence. Write concise completed-work resume prose, normally "
        "18-32 words, with action + technical implementation + credible engineering impact. These "
        "drafts are candidate-attestation-gated by the application, so source must be AI_SUGGESTED "
        "and risk_level HIGH. Avoid duplicate scenarios already present in existing_experience_suggestions."
    )


def _request_supports(
    request: TailoringRequest,
    result: TailoringResponse,
    unsupported_skills: list[str],
    count: int,
) -> list[TailoringSuggestion]:
    response = structured_completion(
        _system_prompt(count),
        json.dumps(_payload(request, result, unsupported_skills), indent=2),
        _response_model(count),
        model_env_var="OPENAI_TAILORING_MODEL",
    )
    return list(response.experience_suggestions)


def ensure_max_match_support_ai(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Guarantee a small support-bullet set when MAX_MATCH emits skill cards."""
    if request.mode != "MAX_MATCH" or not result.skill_suggestions or not request.experiences:
        return result

    card_map = _skill_card_map(result)
    card_keys = set(card_map)
    target = _target_support_count(len(card_map))
    current_count = _supporting_bullet_count(result, card_keys)
    needed = max(0, target - current_count)
    if needed == 0:
        return result

    supported = _supported_skill_keys(result, card_keys)
    unsupported = [display for key, display in card_map.items() if key not in supported]
    if not unsupported:
        return result

    candidates = _request_supports(request, result, unsupported, needed)
    valid = _validate_supports(request, result, candidates, needed)
    result.experience_suggestions.extend(valid)

    remaining = needed - len(valid)
    if remaining <= 0:
        return result

    # A malformed anchor or duplicate can invalidate a model suggestion even
    # when the prose is good. Retry once with the still-unsupported cards
    # instead of silently falling back to a skills-only MAX_MATCH result.
    supported = _supported_skill_keys(result, card_keys)
    unsupported = [display for key, display in card_map.items() if key not in supported]
    if not unsupported:
        return result

    retry_candidates = _request_supports(request, result, unsupported, remaining)
    retry_valid = _validate_supports(request, result, retry_candidates, remaining)
    result.experience_suggestions.extend(retry_valid)
    return result


def ensure_max_match_support_best_effort(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Keep the successful primary draft if the support-recovery call fails."""
    try:
        return ensure_max_match_support_ai(request, result)
    except AIProviderError:
        logger.warning("MAX_MATCH support-bullet recovery failed", exc_info=True)
        return result
