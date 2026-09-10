"""Pydantic schemas for structured resume parsing output."""

from __future__ import annotations

from typing import Annotated

from pydantic import BaseModel, BeforeValidator, Field, WithJsonSchema


def _normalize_string_map(value: object) -> dict[str, str]:
    """Accept either the application's map or OpenAI's fixed assignment list."""
    if isinstance(value, list):
        converted: dict[str, str] = {}
        for item in value:
            if isinstance(item, BaseModel):
                item = item.model_dump()
            if not isinstance(item, dict):
                continue
            skill = str(item.get("skill", "")).strip()
            category = str(item.get("category", "")).strip()
            if skill:
                converted[skill] = category
        return converted
    if not isinstance(value, dict):
        return {}
    return {
        str(skill).strip(): str(category).strip()
        for skill, category in value.items()
        if str(skill).strip()
    }


_STRING_MAP_WIRE_SCHEMA = {
    "type": "array",
    "items": {
        "type": "object",
        "properties": {
            "skill": {"type": "string"},
            "category": {"type": "string"},
        },
        "required": ["skill", "category"],
        "additionalProperties": False,
    },
}

StringMap = Annotated[
    dict[str, str],
    BeforeValidator(_normalize_string_map),
    WithJsonSchema(_STRING_MAP_WIRE_SCHEMA),
]


class ContactInfo(BaseModel):
    name: str | None = None
    headline: str | None = None
    email: str | None = None
    phone: str | None = None
    location: str | None = None
    linkedin_url: str | None = None


class ExperienceEntry(BaseModel):
    company: str | None = None
    title: str | None = None
    start_date: str | None = None
    end_date: str | None = None
    location: str | None = None
    bullets: list[str] = Field(default_factory=list)
    detected_skills: list[str] = Field(default_factory=list)
    technologies: list[str] = Field(default_factory=list)


class ResumeProfile(BaseModel):
    contact: ContactInfo = Field(default_factory=ContactInfo)
    summary: str | None = None
    skills: list[str] = Field(default_factory=list)
    # Runtime stays a dict for the renderer/API. The validation schema exposes
    # a fixed assignment list so OpenAI strict Structured Outputs can parse it.
    skill_categories: StringMap = Field(default_factory=dict)
    experiences: list[ExperienceEntry] = Field(default_factory=list)
    education: list[str] = Field(default_factory=list)
    certifications: list[str] = Field(default_factory=list)


class ExtractRequest(BaseModel):
    mime_type: str


class ExtractResponse(BaseModel):
    raw_text: str


class ParseRequest(BaseModel):
    raw_text: str


class ParseResponse(BaseModel):
    profile: ResumeProfile
