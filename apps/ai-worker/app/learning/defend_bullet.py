"""Defend This Bullet: likely interview questions for a resume bullet (§32).

Draws from the same curated content bank as Quick Prep, keyed by the skills
mentioned in the bullet. Deterministic heuristic, not a real LLM call.
"""

from __future__ import annotations

from app.learning.content_bank import lookup
from app.learning.models import DefendBulletRequest, DefendBulletResponse, InterviewQuestion

_MAX_QUESTIONS = 8


def defend_bullet(request: DefendBulletRequest) -> DefendBulletResponse:
    questions: list[InterviewQuestion] = []
    seen: set[str] = set()

    for skill in request.skills:
        entry = lookup(skill)
        if not entry:
            continue
        for q in entry.get("questions", []):
            if q["question"] in seen:
                continue
            seen.add(q["question"])
            questions.append(InterviewQuestion(**q))
            if len(questions) >= _MAX_QUESTIONS:
                return DefendBulletResponse(questions=questions)

    if not questions:
        questions.append(
            InterviewQuestion(
                question="Walk me through this bullet in more detail.",
                concise_answer=(
                    "Describe the problem, your specific contribution, the technologies "
                    "used, and the measurable outcome."
                ),
                deeper_explanation=(
                    "No curated question bank matched the skills in this bullet. Prepare "
                    "a STAR-style (Situation, Task, Action, Result) explanation covering "
                    "what you personally built."
                ),
            )
        )

    return DefendBulletResponse(questions=questions)
