from app.learning.learning_plan import generate_learning_plan
from app.learning.models import LearningPlanRequest


def test_learning_plan_deduplicates_missing_skills_and_projects() -> None:
    response = generate_learning_plan(
        LearningPlanRequest(
            job_title="Platform Engineer",
            missing_skills=[
                "Docker and Kubernetes",
                "Docker and Kubernetes",
                "  docker and kubernetes  ",
            ],
            current_readiness=40,
            target_readiness=70,
        )
    )

    assert response.skills == ["Docker and Kubernetes"]
    assert response.projects == [
        "Build a small project using Docker and Kubernetes to reinforce hands-on understanding."
    ]
    assert response.estimated_effort_category == "QUICK_PREP"
