"""Pydantic schemas for Quick Prep, Defend This Bullet, and learning plans."""

from __future__ import annotations

from pydantic import BaseModel, Field


class InterviewQuestion(BaseModel):
    question: str
    concise_answer: str
    deeper_explanation: str


class QuickPrepModule(BaseModel):
    skill: str
    what_it_is: str
    why_it_matters: str
    transferable_from: list[str] = Field(default_factory=list)
    core_concepts: list[str] = Field(default_factory=list)
    screening_points: list[str] = Field(default_factory=list)
    interview_questions: list[InterviewQuestion] = Field(default_factory=list)
    common_mistakes: list[str] = Field(default_factory=list)
    architecture_questions: list[str] = Field(default_factory=list)
    example_code: str | None = None


class QuickPrepRequest(BaseModel):
    skill: str
    transferable_from: list[str] = Field(default_factory=list)


class DefendBulletRequest(BaseModel):
    bullet_text: str
    skills: list[str] = Field(default_factory=list)


class DefendBulletResponse(BaseModel):
    questions: list[InterviewQuestion] = Field(default_factory=list)


class LearningPlanRequest(BaseModel):
    job_title: str
    missing_skills: list[str] = Field(default_factory=list)
    current_readiness: int = 0
    target_readiness: int = 0


class LearningPlanResponse(BaseModel):
    skills: list[str] = Field(default_factory=list)
    current_readiness: int = 0
    target_readiness: int = 0
    topics: list[str] = Field(default_factory=list)
    practice_questions: list[InterviewQuestion] = Field(default_factory=list)
    projects: list[str] = Field(default_factory=list)
    architecture_questions: list[str] = Field(default_factory=list)
    estimated_effort_category: str = "STANDARD_PREP"
