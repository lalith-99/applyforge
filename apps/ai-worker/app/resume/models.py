"""Pydantic schemas for structured resume parsing output."""

from __future__ import annotations

from pydantic import BaseModel, Field


class ContactInfo(BaseModel):
    name: str | None = None
    email: str | None = None
    phone: str | None = None
    location: str | None = None


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
