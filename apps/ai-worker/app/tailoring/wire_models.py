"""OpenAI Structured Outputs wire schemas for resume tailoring.

Domain/API models intentionally use maps where that is convenient for the
application. OpenAI strict Structured Outputs do not support arbitrary-key
objects such as ``dict[str, SkillCategory]`` reliably. Keep that concern at the
provider boundary: the model returns fixed-shape ``[{skill, category}]`` rows,
then we convert them into the existing domain representation.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, field_validator

from app.tailoring.models import (
    SkillCategory,
    TailoringResponse,
    TailoringSuggestion,
    _canonical_skill_display,
)


class SkillCategoryAssignment(BaseModel):
    skill: str
    category: SkillCategory

    @field_validator("skill", mode="before")
    @classmethod
    def _normalize_skill(cls, value: object) -> str:
        return _canonical_skill_display(str(value))


class AITailoringSuggestion(BaseModel):
    """Fixed-shape suggestion schema safe for OpenAI strict JSON schema."""

    section: Literal["summary", "skills", "experience"]
    original_text: str | None
    suggested_text: str
    requirements_addressed: list[str]
    skills_added: list[str]
    keywords_added: list[str]
    skill_categories: list[SkillCategoryAssignment]
    operation: Literal["REWRITE", "ADD"]
    target_company: str | None
    target_title: str | None
    source: Literal["MASTER_RESUME", "AI_SUGGESTED"]
    reason: str
    confidence: float
    risk_level: Literal["LOW", "MEDIUM", "HIGH"]

    @field_validator("operation", mode="before")
    @classmethod
    def _normalize_operation(cls, value: object) -> str:
        upper = str(value).upper()
        return upper if upper in {"REWRITE", "ADD"} else "REWRITE"

    @field_validator("risk_level", mode="before")
    @classmethod
    def _normalize_risk(cls, value: object) -> str:
        upper = str(value).upper()
        return upper if upper in {"LOW", "MEDIUM", "HIGH"} else "LOW"

    def to_domain(self) -> TailoringSuggestion:
        return TailoringSuggestion(
            section=self.section,
            original_text=self.original_text,
            suggested_text=self.suggested_text,
            requirements_addressed=self.requirements_addressed,
            skills_added=self.skills_added,
            keywords_added=self.keywords_added,
            skill_categories={item.skill: item.category for item in self.skill_categories},
            operation=self.operation,
            target_company=self.target_company,
            target_title=self.target_title,
            source=self.source,
            reason=self.reason,
            confidence=self.confidence,
            risk_level=self.risk_level,
        )


class AITailoringResponse(BaseModel):
    """Provider-facing form of TailoringResponse with no dynamic maps/defaults."""

    summary_suggestion: AITailoringSuggestion | None
    skill_suggestions: list[AITailoringSuggestion]
    experience_suggestions: list[AITailoringSuggestion]
    keyword_coverage_before: float
    keyword_coverage_after: float

    def to_domain(self) -> TailoringResponse:
        return TailoringResponse(
            summary_suggestion=(
                self.summary_suggestion.to_domain() if self.summary_suggestion else None
            ),
            skill_suggestions=[item.to_domain() for item in self.skill_suggestions],
            experience_suggestions=[
                item.to_domain() for item in self.experience_suggestions
            ],
            keyword_coverage_before=self.keyword_coverage_before,
            keyword_coverage_after=self.keyword_coverage_after,
        )


class AITailoringEnrichmentResponse(BaseModel):
    skill_categories: list[SkillCategoryAssignment]
    experience_suggestions: list[AITailoringSuggestion]
