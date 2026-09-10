"""Learning plan generation: aggregates Quick Prep content for a job's
missing skills into a single "Prepare for This Job" plan (§34)."""

from __future__ import annotations

from app.learning.content_bank import lookup
from app.learning.models import InterviewQuestion, LearningPlanRequest, LearningPlanResponse

_MAX_PRACTICE_QUESTIONS = 8


def _unique_skills(values: list[str]) -> list[str]:
    """Return normalized, case-insensitively unique skills in input order."""
    seen: set[str] = set()
    unique: list[str] = []
    for value in values:
        cleaned = " ".join(value.split())
        key = cleaned.casefold()
        if not cleaned or key in seen:
            continue
        seen.add(key)
        unique.append(cleaned)
    return unique


def generate_learning_plan(request: LearningPlanRequest) -> LearningPlanResponse:
    topics: list[str] = []
    practice_questions: list[InterviewQuestion] = []
    architecture_questions: list[str] = []
    projects: list[str] = []
    seen_questions: set[str] = set()
    missing_skills = _unique_skills(request.missing_skills)

    for skill in missing_skills:
        entry = lookup(skill)
        if entry:
            topics.extend(entry.get("core_concepts", []))
            architecture_questions.extend(entry.get("architecture_questions", []))
            for q in entry.get("questions", []):
                if (
                    q["question"] not in seen_questions
                    and len(practice_questions) < _MAX_PRACTICE_QUESTIONS
                ):
                    seen_questions.add(q["question"])
                    practice_questions.append(InterviewQuestion(**q))
        else:
            topics.append(f"Core {skill} concepts and terminology")
        projects.append(f"Build a small project using {skill} to reinforce hands-on understanding.")

    if len(missing_skills) <= 2:
        effort = "QUICK_PREP"
    elif len(missing_skills) <= 5:
        effort = "STANDARD_PREP"
    else:
        effort = "DEEPER_GAP"

    return LearningPlanResponse(
        skills=missing_skills,
        current_readiness=request.current_readiness,
        target_readiness=request.target_readiness,
        topics=list(dict.fromkeys(topics)),
        practice_questions=practice_questions,
        projects=list(dict.fromkeys(projects)),
        architecture_questions=list(dict.fromkeys(architecture_questions)),
        estimated_effort_category=effort,
    )
