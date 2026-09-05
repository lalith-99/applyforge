"""Tests for heuristic resume tailoring suggestion generation."""

from app.tailoring.heuristics import generate_tailoring
from app.tailoring.models import ExperienceInput, TailoringRequest, TransferableMatchInput

EXPERIENCES = [
    ExperienceInput(
        company="Acme",
        title="Backend Engineer",
        bullets=["Built Go microservices for telecom file processing."],
        detected_skills=["Go", "Kafka"],
    )
]


def _request(mode: str, transfers=None) -> TailoringRequest:
    return TailoringRequest(
        mode=mode,
        job_title="Senior Backend Engineer",
        master_skills=["Go", "Kafka", "PostgreSQL"],
        master_summary="Backend engineer with distributed systems experience.",
        experiences=EXPERIENCES,
        required_skills=["Go", "Amazon SQS"],
        preferred_skills=["Kubernetes"],
        responsibilities=[],
        transferable_matches=transfers or [],
    )


def test_strict_mode_never_suggests_new_skills() -> None:
    response = generate_tailoring(_request("STRICT"))
    assert response.skill_suggestions == []


def test_growth_mode_only_suggests_skills_with_transfer_support() -> None:
    transfers = [
        TransferableMatchInput(
            source_skill="Kafka",
            target_skill="Amazon SQS",
            level="MEDIUM",
            prep_classification="QUICK_PREP",
        )
    ]
    response = generate_tailoring(_request("GROWTH", transfers))

    added = {s.skills_added[0] for s in response.skill_suggestions}
    assert "Amazon SQS" in added
    # Kubernetes has no transfer data in this fixture, so GROWTH should skip it.
    assert "Kubernetes" not in added
    for s in response.skill_suggestions:
        assert s.source == "AI_SUGGESTED"


def test_max_match_mode_suggests_all_missing_skills() -> None:
    response = generate_tailoring(_request("MAX_MATCH"))
    added = {s.skills_added[0] for s in response.skill_suggestions}
    assert "Amazon SQS" in added
    assert "Kubernetes" in added


def test_skill_suggestion_with_transfer_has_lower_risk_than_without() -> None:
    transfers = [
        TransferableMatchInput(
            source_skill="Kafka",
            target_skill="Amazon SQS",
            level="MEDIUM",
            prep_classification="QUICK_PREP",
        )
    ]
    response = generate_tailoring(_request("MAX_MATCH", transfers))

    by_skill = {s.skills_added[0]: s for s in response.skill_suggestions}
    assert by_skill["Amazon SQS"].risk_level == "LOW"
    assert by_skill["Kubernetes"].risk_level == "MEDIUM"


def test_summary_suggestion_highlights_matched_skills() -> None:
    response = generate_tailoring(_request("GROWTH"))
    assert response.summary_suggestion is not None
    assert "Go" in response.summary_suggestion.suggested_text


def test_experience_suggestion_only_uses_existing_experience() -> None:
    response = generate_tailoring(_request("GROWTH"))
    assert len(response.experience_suggestions) == 1
    suggestion = response.experience_suggestions[0]
    assert suggestion.source == "MASTER_RESUME"
    assert "go" in suggestion.requirements_addressed


def test_keyword_coverage_improves_with_more_permissive_modes() -> None:
    strict = generate_tailoring(_request("STRICT"))
    max_match = generate_tailoring(_request("MAX_MATCH"))
    assert max_match.keyword_coverage_after >= strict.keyword_coverage_after
