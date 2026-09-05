"""Tests for Quick Prep, Defend This Bullet, and learning-plan heuristics."""

from app.learning.defend_bullet import defend_bullet
from app.learning.learning_plan import generate_learning_plan
from app.learning.models import DefendBulletRequest, LearningPlanRequest, QuickPrepRequest
from app.learning.quick_prep import generate_quick_prep


def test_quick_prep_uses_curated_content_for_known_skill() -> None:
    module = generate_quick_prep(QuickPrepRequest(skill="Amazon SQS", transferable_from=["Kafka"]))
    assert module.skill == "Amazon SQS"
    assert "Kafka" in module.transferable_from
    assert len(module.interview_questions) > 0
    assert module.what_it_is != ""


def test_quick_prep_falls_back_to_generic_for_unknown_skill() -> None:
    module = generate_quick_prep(QuickPrepRequest(skill="SomeObscureFramework"))
    assert module.skill == "SomeObscureFramework"
    assert len(module.interview_questions) == 1
    assert "SomeObscureFramework" in module.what_it_is


def test_defend_bullet_returns_questions_for_known_skills() -> None:
    response = defend_bullet(
        DefendBulletRequest(bullet_text="Built SQS pipelines", skills=["Amazon SQS"])
    )
    assert len(response.questions) > 0
    assert any("SQS" in q.question or "sqs" in q.question.lower() for q in response.questions)


def test_defend_bullet_falls_back_when_no_known_skills() -> None:
    response = defend_bullet(
        DefendBulletRequest(bullet_text="Did some obscure thing", skills=["UnknownTech"])
    )
    assert len(response.questions) == 1


def test_defend_bullet_deduplicates_questions_across_skills() -> None:
    response = defend_bullet(DefendBulletRequest(bullet_text="x", skills=["Kafka", "Kafka"]))
    questions = [q.question for q in response.questions]
    assert len(questions) == len(set(questions))


def test_learning_plan_classifies_effort_by_gap_size() -> None:
    small = generate_learning_plan(
        LearningPlanRequest(job_title="Backend Engineer", missing_skills=["Go"])
    )
    large = generate_learning_plan(
        LearningPlanRequest(
            job_title="Backend Engineer", missing_skills=["A", "B", "C", "D", "E", "F"]
        )
    )
    assert small.estimated_effort_category == "QUICK_PREP"
    assert large.estimated_effort_category == "DEEPER_GAP"


def test_learning_plan_aggregates_topics_from_known_skills() -> None:
    plan = generate_learning_plan(
        LearningPlanRequest(job_title="Backend Engineer", missing_skills=["Kubernetes", "Docker"])
    )
    assert len(plan.topics) > 0
    assert len(plan.architecture_questions) > 0
