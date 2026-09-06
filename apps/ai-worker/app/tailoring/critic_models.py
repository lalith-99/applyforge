"""Pydantic schemas for the AI resume-tailoring critic (Phase K): reviews a
generated set of suggestions against the master resume and job requirements
for unsupported claims, weak bullets, missing keywords, and an ATS-style
score, so a poor first pass can trigger one bounded revision instead of
shipping unreviewed output.
"""

from __future__ import annotations

from pydantic import BaseModel, Field


class SuggestionInput(BaseModel):
    section: str
    suggested_text: str
    skills_added: list[str] = Field(default_factory=list)
    source: str = ""
    risk_level: str = "LOW"


class CritiqueRequest(BaseModel):
    job_title: str
    master_resume_summary: str | None = None
    master_skills: list[str] = Field(default_factory=list)
    required_skills: list[str] = Field(default_factory=list)
    preferred_skills: list[str] = Field(default_factory=list)
    suggestions: list[SuggestionInput] = Field(default_factory=list)


class CritiqueResult(BaseModel):
    unsupported_claims: list[str] = Field(default_factory=list)
    missing_high_value_keywords: list[str] = Field(default_factory=list)
    weak_bullets: list[str] = Field(default_factory=list)
    repetition: list[str] = Field(default_factory=list)
    ats_score: int = 0
    technical_match_score: int = 0
    human_readability: int = 0
    recommend_regeneration: bool = False
    feedback: str = ""


class CritiqueResponse(BaseModel):
    result: CritiqueResult
