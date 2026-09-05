"""Pydantic schemas for structured job-description parsing output."""

from __future__ import annotations

from pydantic import BaseModel, Field


class SkillRequirement(BaseModel):
    normalized_name: str
    original_text: str
    importance: str  # "required" | "preferred"
    category: str = "technology"
    confidence: float = 0.6


class JobRequirements(BaseModel):
    role_family: str | None = None
    normalized_title: str | None = None
    seniority: str | None = None
    required_skills: list[SkillRequirement] = Field(default_factory=list)
    preferred_skills: list[SkillRequirement] = Field(default_factory=list)
    required_experience_years: int | None = None
    responsibilities: list[str] = Field(default_factory=list)
    domains: list[str] = Field(default_factory=list)
    education_requirements: list[str] = Field(default_factory=list)
    certifications: list[str] = Field(default_factory=list)
    clearance_requirements: str | None = None
    work_authorization_requirements: str | None = None
    keywords: list[str] = Field(default_factory=list)


class ParseRequirementsRequest(BaseModel):
    title: str
    description: str


class ParseRequirementsResponse(BaseModel):
    requirements: JobRequirements
