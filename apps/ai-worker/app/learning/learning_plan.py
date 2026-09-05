"""Learning plan generation: aggregates Quick Prep content for a job's
missing skills into a single "Prepare for This Job" plan (§34)."""

from __future__ import annotations

from app.learning.content_bank import lookup
from app.learning.models import InterviewQuestion, LearningPlanRequest, LearningPlanResponse

_MAX_PRACTICE_QUESTIONS = 8


def generate_learning_plan(request: LearningPlanRequest) -> LearningPlanResponse:
    topics: list[str] = []
    practice_questions: list[InterviewQuestion] = []
    architecture_questions: list[str] = []
    projects: list[str] = []
    seen_questions: set[str] = set()

    for skill in request.missing_skills:
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

    if len(request.missing_skills) <= 2:
        effort = "QUICK_PREP"
    elif len(request.missing_skills) <= 5:
        effort = "STANDARD_PREP"
    else:
        effort = "DEEPER_GAP"

    return LearningPlanResponse(
        skills=request.missing_skills,
        current_readiness=request.current_readiness,
        target_readiness=request.target_readiness,
        topics=list(dict.fromkeys(topics)),
        practice_questions=practice_questions,
        projects=projects,
        architecture_questions=list(dict.fromkeys(architecture_questions)),
        estimated_effort_category=effort,
    )
