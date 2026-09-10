"""Focused post-tailoring enrichment for skill placement and missing support.

The first tailoring pass remains authoritative. This module never replaces it;
it only assigns explicit skill categories and optionally adds a very small
number of additional experience suggestions when important JD requirements are
still weakly represented.
"""

from __future__ import annotations

import json
from collections import Counter

from pydantic import BaseModel, Field

from app.providers.openai_provider import AIProviderError, structured_completion
from app.tailoring.models import (
    SKILL_CATEGORIES,
    SkillCategory,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
)


class TailoringEnrichmentResponse(BaseModel):
    skill_categories: dict[str, SkillCategory] = Field(default_factory=dict)
    experience_suggestions: list[TailoringSuggestion] = Field(default_factory=list)


def _skill_key(value: str) -> str:
    key = " ".join(str(value).strip().lower().split())
    aliases = {
        "golang": "go",
        "go (golang)": "go",
        "react.js": "react",
        "reactjs": "react",
        "node js": "nodejs",
        "node.js": "nodejs",
        "amazon web services": "aws",
        "apache kafka": "kafka",
        "argo cd": "argocd",
    }
    return aliases.get(key, key)


def _fallback_category(skill: str) -> str:
    """Deterministic safety net only when the AI omits a category."""
    value = _skill_key(skill)
    language_values = {
        "java",
        "python",
        "javascript",
        "typescript",
        "sql",
        "bash",
        "kotlin",
        "go",
        "rust",
        "swift",
        "c",
        "c++",
        "c#",
    }
    if value in language_values:
        return "Languages"
    if any(
        token in value
        for token in (
            "prompt",
            "agent",
            "mcp",
            "model context protocol",
            "tool calling",
            "function calling",
            "rag",
            "retrieval-augmented",
            "langchain",
            "bedrock",
            "openai",
            "llm",
            "semantic retrieval",
            "claude",
            "cursor",
            "ai observability",
        )
    ):
        return "AI / GenAI"
    if any(
        token in value
        for token in (
            "test",
            "junit",
            "selenium",
            "jest",
            "tdd",
            "jira",
            "code review",
        )
    ):
        return "Testing & Tools"
    if any(
        token in value
        for token in (
            "aws",
            "azure",
            "gcp",
            "kubernetes",
            "docker",
            "terraform",
            "cloudformation",
            "jenkins",
            "ci/cd",
            "linux",
            "helm",
            "argocd",
            "monitoring",
            "observability",
        )
    ):
        return "Cloud & DevOps"
    if any(
        token in value for token in ("oauth", "saml", "rbac", "security", "pci")
    ):
        return "Security"
    if any(
        token in value
        for token in ("spring", "rest", "microservice", "jpa", "hibernate")
    ):
        return "Backend"
    if any(
        token in value
        for token in ("kafka", "redis", "postgres", "mysql", "oracle", "database")
    ):
        return "Databases & Messaging"
    if any(
        token in value for token in ("angular", "react", "html", "css", "frontend")
    ):
        return "Frontend"
    return "Other"


def _exact_role_exists(
    request: TailoringRequest, company: str | None, title: str | None
) -> bool:
    if not company and not title:
        return False
    for exp in request.experiences:
        company_ok = (
            not company
            or (exp.company or "").strip().casefold() == company.strip().casefold()
        )
        title_ok = (
            not title
            or (exp.title or "").strip().casefold() == title.strip().casefold()
        )
        if company_ok and title_ok:
            return True
    return False


def _enrichment_payload(
    request: TailoringRequest, result: TailoringResponse
) -> dict[str, object]:
    return {
        "job_title": request.job_title,
        "job_description": request.job_description,
        "required_skills": request.required_skills,
        "preferred_skills": request.preferred_skills,
        "responsibilities": request.responsibilities,
        "master_skills": request.master_skills,
        "experiences": [exp.model_dump() for exp in request.experiences],
        "current_skill_suggestions": [
            suggestion.model_dump() for suggestion in result.skill_suggestions
        ],
        "current_experience_suggestions": [
            suggestion.model_dump() for suggestion in result.experience_suggestions
        ],
    }


def _normalize_category_map(
    result: TailoringResponse,
    enrichment: TailoringEnrichmentResponse,
) -> None:
    by_key = {
        _skill_key(skill): category
        for skill, category in enrichment.skill_categories.items()
        if category in SKILL_CATEGORIES
    }
    for suggestion in result.skill_suggestions:
        categories: dict[str, str] = {}
        for skill in suggestion.skills_added:
            categories[skill] = by_key.get(_skill_key(skill), _fallback_category(skill))
        suggestion.skill_categories = categories


def _validated_extra_suggestions(
    request: TailoringRequest,
    result: TailoringResponse,
    enrichment: TailoringEnrichmentResponse,
) -> list[TailoringSuggestion]:
    source_bullets = {
        bullet for experience in request.experiences for bullet in experience.bullets
    }
    used_originals = {
        suggestion.original_text
        for suggestion in result.experience_suggestions
        if suggestion.original_text
    }
    added_per_role: Counter[tuple[str, str]] = Counter()
    extras: list[TailoringSuggestion] = []

    for suggestion in enrichment.experience_suggestions:
        if len(extras) >= 2:
            break
        suggestion.section = "experience"

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

        # Every enrichment-pass experience draft requires explicit candidate
        # review. This prevents imperfect model metadata from accidentally
        # turning second-pass prose into verified employment history.
        suggestion.source = "AI_SUGGESTED"
        suggestion.risk_level = "HIGH"

        normalized_categories: dict[str, str] = {}
        for skill in suggestion.skills_added:
            category = next(
                (
                    value
                    for key, value in suggestion.skill_categories.items()
                    if _skill_key(key) == _skill_key(skill)
                    and value in SKILL_CATEGORIES
                ),
                _fallback_category(skill),
            )
            normalized_categories[skill] = category
        suggestion.skill_categories = normalized_categories
        extras.append(suggestion)

    return extras


def enrich_tailoring_ai(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Enrich a successful first-pass draft without replacing its content."""
    if request.mode != "MAX_MATCH" and not result.skill_suggestions:
        return result

    system = (
        "You are the placement and support editor for a technical resume. The first "
        "tailoring pass has already produced the primary draft. NEVER replace, summarize, "
        "or rewrite that whole draft. Your job has exactly two responsibilities. First, "
        "classify every skill currently proposed in current_skill_suggestions into exactly "
        "one resume category: Languages, Backend, Databases & Messaging, Cloud & DevOps, "
        "Frontend, Security, Testing & Tools, AI / GenAI, or Other. Put modern agentic-AI "
        "capabilities such as prompt engineering, tool/function calling, agent memory, agent "
        "orchestration, agent frameworks, MCP/Model Context Protocol, AI evaluation/observability, "
        "Claude Code, Cursor, RAG, retrieval/context pipelines, LLMs, Bedrock, and OpenAI APIs "
        "under AI / GenAI unless the term is clearly a general testing/devops tool instead. "
        "Put Go/Golang and other programming languages under Languages. Second, inspect the "
        "full JD, master experiences, and current experience suggestions. Return at most two "
        "ADDITIONAL high-value experience suggestions only when an important job requirement "
        "is still weakly supported. Prefer REWRITE: operation='REWRITE', original_text must "
        "exactly equal one unused existing master bullet, and target_company/target_title must "
        "be null. Use operation='ADD' only when no existing bullet can naturally carry the "
        "requirement; then original_text must be null and target_company/target_title must "
        "exactly match an existing role. Never create a new employer, title, date, certification, "
        "metric, team size, or named business outcome. Any ADD bullet is an AI_SUGGESTED/HIGH "
        "candidate-attestation draft, not verified employment history. For new target technology "
        "in any extra bullet, include it in skills_added and assign its category in "
        "skill_categories. Keep extra bullets concise, technical, and interview-defensible. "
        "Do not append keywords to unrelated work. Do not emit learning/proficiency/disclaimer "
        "language inside resume prose."
    )

    enrichment = structured_completion(
        system,
        json.dumps(_enrichment_payload(request, result), indent=2),
        TailoringEnrichmentResponse,
        model_env_var="OPENAI_TAILORING_MODEL",
    )
    _normalize_category_map(result, enrichment)
    result.experience_suggestions.extend(
        _validated_extra_suggestions(request, result, enrichment)
    )
    return result


def enrich_tailoring_best_effort(
    request: TailoringRequest, result: TailoringResponse
) -> TailoringResponse:
    """Never discard a good first draft because enrichment itself fails."""
    try:
        return enrich_tailoring_ai(request, result)
    except AIProviderError:
        for suggestion in result.skill_suggestions:
            suggestion.skill_categories = {
                skill: _fallback_category(skill) for skill in suggestion.skills_added
            }
        return result
