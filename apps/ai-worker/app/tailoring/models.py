"""Pydantic schemas for AI resume tailoring output."""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field, field_validator

TAILORING_MODES = ("STRICT", "GROWTH", "MAX_MATCH")
_RISK_LEVELS = ("LOW", "MEDIUM", "HIGH")


def _canonical_skill_display(value: str) -> str:
    """Normalize model-generated skill labels before they reach final rendering.

    The master resume keeps its source-faithful spelling, but AI suggestions
    should use one canonical display name so variants such as "Go (Golang)"
    do not fall into the renderer's generic Other category.
    """
    stripped = str(value).strip()
    aliases = {
        "golang": "Go",
        "go (golang)": "Go",
        "reactjs": "React",
        "react.js": "React",
        "node js": "Node.js",
        "nodejs": "Node.js",
        "amazon web services": "AWS",
        "apache kafka": "Kafka",
        "argo cd": "ArgoCD",
    }
    return aliases.get(stripped.lower(), stripped)


class ExperienceInput(BaseModel):
    company: str | None = None
    title: str | None = None
    bullets: list[str] = Field(default_factory=list)
    detected_skills: list[str] = Field(default_factory=list)


class TransferableMatchInput(BaseModel):
    source_skill: str
    target_skill: str
    level: str
    prep_classification: str


class TailoringRequest(BaseModel):
    mode: str
    job_title: str
    job_description: str = ""
    master_skills: list[str] = Field(default_factory=list)
    master_summary: str | None = None
    experiences: list[ExperienceInput] = Field(default_factory=list)
    required_skills: list[str] = Field(default_factory=list)
    preferred_skills: list[str] = Field(default_factory=list)
    responsibilities: list[str] = Field(default_factory=list)
    transferable_matches: list[TransferableMatchInput] = Field(default_factory=list)


class TailoringSuggestion(BaseModel):
    section: Literal["summary", "skills", "experience"]
    original_text: str | None = None
    suggested_text: str
    requirements_addressed: list[str] = Field(default_factory=list)
    skills_added: list[str] = Field(default_factory=list)
    keywords_added: list[str] = Field(default_factory=list)
    source: Literal["MASTER_RESUME", "AI_SUGGESTED"]
    reason: str
    confidence: float = 0.6
    risk_level: Literal["LOW", "MEDIUM", "HIGH"] = "LOW"

    @field_validator("skills_added", "keywords_added", mode="before")
    @classmethod
    def _normalize_skill_labels(cls, value: object) -> object:
        if not isinstance(value, list):
            return value
        return [_canonical_skill_display(item) for item in value]

    @field_validator("risk_level", mode="before")
    @classmethod
    def _normalize_risk_level(cls, value: str) -> str:
        # The Go API's DB schema has a hard CHECK constraint on exact
        # uppercase values; an LLM isn't guaranteed to match that casing.
        upper = str(value).upper()
        return upper if upper in _RISK_LEVELS else "LOW"


class TailoringResponse(BaseModel):
    summary_suggestion: TailoringSuggestion | None = None
    skill_suggestions: list[TailoringSuggestion] = Field(default_factory=list)
    experience_suggestions: list[TailoringSuggestion] = Field(default_factory=list)
    keyword_coverage_before: float = 0.0
    keyword_coverage_after: float = 0.0


class ExperienceSupportResponse(BaseModel):
    """Focused repair-pass output for missing skill-to-experience coverage."""

    experience_suggestions: list[TailoringSuggestion] = Field(default_factory=list)
