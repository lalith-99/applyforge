"""Category-aware wrapper around the deterministic resume renderer.

Tailoring can provide explicit skill-category choices from the AI model. The
underlying renderer remains deterministic and owns layout/page fitting; this
wrapper only supplies per-request category overrides. ContextVar keeps the
override request-local even when document endpoints run concurrently.
"""

from __future__ import annotations

from contextvars import ContextVar

from app.documents import generator
from app.resume.models import ResumeProfile

_VALID_CATEGORIES = {
    "Languages",
    "Backend",
    "Databases & Messaging",
    "Cloud & DevOps",
    "Frontend",
    "Security",
    "Testing & Tools",
    "AI / GenAI",
    "Other",
}
_category_overrides: ContextVar[dict[str, str]] = ContextVar(
    "resume_skill_category_overrides", default={}
)
_original_skill_category = generator._skill_category


def _enhanced_fallback(skill: str) -> str:
    """Cover modern categories when a legacy resume has no explicit AI map."""
    value = skill.lower().strip()
    if any(
        token in value
        for token in (
            "prompt engineering",
            "prompt design",
            "tool calling",
            "function calling",
            "agent memory",
            "agent orchestration",
            "agent framework",
            "agentic",
            "model context protocol",
            "mcp server",
            "mcp",
            "ai observability",
            "ai evaluation",
            "retrieval-augmented generation",
            "context pipeline",
            "cursor",
        )
    ):
        return "AI / GenAI"
    if any(
        token in value
        for token in (
            "automated testing framework",
            "api testing",
            "integration testing",
            "data pipeline testing",
            "test framework",
            "test automation",
        )
    ):
        return "Testing & Tools"
    if any(
        token in value
        for token in (
            "infrastructure as code",
            "cloudformation",
            "monitoring",
            "observability",
        )
    ):
        return "Cloud & DevOps"
    return _original_skill_category(skill)


def _category_for(skill: str) -> str:
    overrides = _category_overrides.get()
    category = overrides.get(skill.casefold())
    if category in _VALID_CATEGORIES:
        return category
    return _enhanced_fallback(skill)


# The renderer resolves this global at call time. Install one context-aware
# resolver once; each wrapper call below only changes its request-local map.
generator._skill_category = _category_for


def _normalized_overrides(profile: ResumeProfile) -> dict[str, str]:
    return {
        str(skill).strip().casefold(): str(category).strip()
        for skill, category in profile.skill_categories.items()
        if str(skill).strip() and str(category).strip() in _VALID_CATEGORIES
    }


def render_pdf(profile: ResumeProfile) -> bytes:
    token = _category_overrides.set(_normalized_overrides(profile))
    try:
        return generator.render_pdf(profile)
    finally:
        _category_overrides.reset(token)


def render_docx(profile: ResumeProfile) -> bytes:
    token = _category_overrides.set(_normalized_overrides(profile))
    try:
        return generator.render_docx(profile)
    finally:
        _category_overrides.reset(token)
