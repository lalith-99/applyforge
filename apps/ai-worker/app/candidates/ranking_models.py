"""Pydantic schemas for AI job-fit ranking (Phase H): given a candidate
summary and a batch of already deterministic-scored/filtered jobs, ask the
model to judge genuine fit - the deterministic scorer (matching package,
Go side) intentionally never delegates to an LLM, so this is a distinct,
later stage layered on top of it, not a replacement.
"""

from __future__ import annotations

from pydantic import BaseModel, Field


class JobRankingInput(BaseModel):
    job_id: str
    title: str
    company_name: str
    seniority: str | None = None
    remote_type: str | None = None
    matched_skills: list[str] = Field(default_factory=list)
    missing_required_skills: list[str] = Field(default_factory=list)
    missing_preferred_skills: list[str] = Field(default_factory=list)
    transferable_notes: list[str] = Field(default_factory=list)
    deterministic_score: int = 0


class RankJobsRequest(BaseModel):
    candidate_summary: str = ""
    target_roles: list[str] = Field(default_factory=list)
    jobs: list[JobRankingInput] = Field(default_factory=list)


class JobRankingResult(BaseModel):
    job_id: str
    fit_score: int = 0  # 0-100
    interview_probability_score: int = 0  # 0-100
    career_alignment: int = 0  # 0-100
    skill_gap_severity: str = "MEDIUM"  # LOW | MEDIUM | HIGH
    strong_evidence: list[str] = Field(default_factory=list)
    gaps: list[str] = Field(default_factory=list)
    recommendation: str = "CONSIDER"  # APPLY_NOW | STRONG_CONSIDER | CONSIDER | SKIP
    reason: str = ""


class RankJobsResult(BaseModel):
    rankings: list[JobRankingResult] = Field(default_factory=list)


class RankJobsResponse(BaseModel):
    result: RankJobsResult
