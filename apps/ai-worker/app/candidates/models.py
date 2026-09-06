"""Pydantic schemas for the CandidateIntelligenceProfile (Phase F): a single
AI-synthesized summary of a candidate, materialized once per resume/profile
change instead of being recomputed ad hoc from scattered tables on every
match/rank request.
"""

from __future__ import annotations

from pydantic import BaseModel, Field


class ExperienceInput(BaseModel):
    company: str | None = None
    title: str | None = None
    bullets: list[str] = Field(default_factory=list)
    technologies: list[str] = Field(default_factory=list)


class CandidateProfileRequest(BaseModel):
    target_roles: list[str] = Field(default_factory=list)
    seniority: str | None = None
    years_experience: int | None = None
    master_skills: list[str] = Field(default_factory=list)
    master_summary: str | None = None
    experiences: list[ExperienceInput] = Field(default_factory=list)
    preferred_industries: list[str] = Field(default_factory=list)
    preferred_technologies: list[str] = Field(default_factory=list)
    work_authorization: str | None = None
    immigration_status: str | None = None


class TransferableSkillSignal(BaseModel):
    skill: str
    evidence: str
    strength: str = "TRANSFERABLE"  # "HIGH" | "TRANSFERABLE" | "LEARNABLE"


class CandidateIntelligenceProfile(BaseModel):
    target_roles: list[str] = Field(default_factory=list)
    seniority: str | None = None
    years_experience: int | None = None
    core_skills: list[str] = Field(default_factory=list)
    secondary_skills: list[str] = Field(default_factory=list)
    transferable_skills: list[TransferableSkillSignal] = Field(default_factory=list)
    domains: list[str] = Field(default_factory=list)
    architecture_strengths: list[str] = Field(default_factory=list)
    leadership_signals: list[str] = Field(default_factory=list)
    experience_evidence: list[str] = Field(default_factory=list)
    summary: str = ""


class CandidateProfileResponse(BaseModel):
    profile: CandidateIntelligenceProfile
