"""Quick Prep module generation (see MASTER_REQUIREMENTS.md §31).

Uses the curated content bank for common technologies, with a generic
fallback template for anything else. This is a deterministic heuristic
implementation, not a real LLM call — same rationale as app/resume/parsing.py.
"""

from __future__ import annotations

from app.learning.content_bank import lookup
from app.learning.models import InterviewQuestion, QuickPrepModule, QuickPrepRequest


def generate_quick_prep(request: QuickPrepRequest) -> QuickPrepModule:
    entry = lookup(request.skill)
    if entry is None:
        return _generic_module(request)

    return QuickPrepModule(
        skill=request.skill,
        what_it_is=entry["what_it_is"],
        why_it_matters=entry["why_it_matters"],
        transferable_from=request.transferable_from,
        core_concepts=entry.get("core_concepts", []),
        screening_points=entry.get("screening_points", []),
        interview_questions=[InterviewQuestion(**q) for q in entry.get("questions", [])],
        common_mistakes=entry.get("common_mistakes", []),
        architecture_questions=entry.get("architecture_questions", []),
        example_code=entry.get("example_code"),
    )


def _generic_module(request: QuickPrepRequest) -> QuickPrepModule:
    skill = request.skill
    transfer_note = (
        f" Your experience with {', '.join(request.transferable_from)} covers related concepts."
        if request.transferable_from
        else ""
    )
    return QuickPrepModule(
        skill=skill,
        what_it_is=f"{skill} is a technology referenced in this job's requirements.",
        why_it_matters=f"This role expects familiarity with {skill}.{transfer_note}",
        transferable_from=request.transferable_from,
        core_concepts=[f"Core {skill} concepts and terminology", f"Common {skill} use cases"],
        screening_points=[f"Be ready to explain what {skill} is and when you'd use it."],
        interview_questions=[
            InterviewQuestion(
                question=f"What is {skill} and when would you use it?",
                concise_answer=(
                    f"Research the fundamentals of {skill} and how it fits typical "
                    "production systems."
                ),
                deeper_explanation=(
                    f"No curated content is available yet for {skill} — treat this as a "
                    "starting point and verify details against official documentation "
                    "before an interview."
                ),
            )
        ],
        common_mistakes=[f"Overstating hands-on {skill} experience you don't yet have"],
        architecture_questions=[f"How would {skill} fit into a broader system design?"],
        example_code=None,
    )
