"""Tests for heuristic resume tailoring suggestion generation."""

from app.tailoring.heuristics import _sanitize_ai_tailoring, generate_tailoring
from app.tailoring.models import (
    ExperienceInput,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
    TransferableMatchInput,
)

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


def test_max_match_keeps_unsupported_skills_out_of_professional_experience() -> None:
    response = generate_tailoring(_request("MAX_MATCH"))
    experience_text = " ".join(s.suggested_text for s in response.experience_suggestions).lower()

    assert "amazon sqs" not in experience_text
    assert "kubernetes" not in experience_text
    assert all("learning" not in s.suggested_text.lower() for s in response.experience_suggestions)
    assert all(
        "proficiency" not in s.suggested_text.lower() for s in response.experience_suggestions
    )


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


def test_max_match_reaches_skill_coverage_without_fake_experience_touchpoints() -> None:
    response = generate_tailoring(_request("MAX_MATCH"))

    assert response.keyword_coverage_after == 1.0
    assert len(response.skill_suggestions) == 2
    experience_text = " ".join(s.suggested_text for s in response.experience_suggestions).lower()
    assert "amazon sqs" not in experience_text
    assert "kubernetes" not in experience_text


def test_equivalent_skill_labels_do_not_create_false_missing_skills() -> None:
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Frontend Engineer",
        master_skills=["React.js", "Java 21", "Spring Boot 3.4", "PostgreSQL"],
        master_summary="Full-stack engineer.",
        experiences=[
            ExperienceInput(
                company="Acme",
                title="Software Engineer",
                bullets=["Built user interfaces with React.js and Java services."],
                detected_skills=["React.js", "Java 21"],
            )
        ],
        required_skills=["React", "Java", "Spring Boot", "Postgres"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )

    response = generate_tailoring(request)

    assert response.skill_suggestions == []
    assert response.keyword_coverage_before == 1.0
    assert response.keyword_coverage_after == 1.0



def test_ai_sanitizer_rejects_learning_style_azure_experience_rewrite() -> None:
    original = (
        "Develop and modernize enterprise healthcare applications using Java 21, "
        "Spring Boot, REST APIs, JPA, Oracle, and Angular 19."
    )
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Software Engineer",
        master_skills=["Java 21", "Spring Boot", "Angular 19"],
        master_summary="Java software engineer.",
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Development Engineer",
                bullets=[original],
                detected_skills=["Java", "Spring Boot", "Angular"],
            )
        ],
        required_skills=["Java", "Azure"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )
    result = TailoringResponse(
        experience_suggestions=[
            TailoringSuggestion(
                section="experience",
                original_text=original,
                suggested_text=(
                    original.rstrip(".")
                    + ", while actively building hands-on proficiency in Azure."
                ),
                requirements_addressed=["Azure"],
                skills_added=["Azure"],
                keywords_added=["Azure"],
                source="AI_SUGGESTED",
                reason="Add Azure keyword.",
                confidence=0.5,
                risk_level="HIGH",
            )
        ],
        skill_suggestions=[],
        keyword_coverage_before=0.5,
        keyword_coverage_after=1.0,
    )

    sanitized = _sanitize_ai_tailoring(request, result)

    assert sanitized.experience_suggestions == []




def test_ai_sanitizer_keeps_strong_azure_draft_but_marks_it_for_attestation() -> None:
    original = (
        "Develop and modernize enterprise healthcare applications using Java 21, "
        "Spring Boot, REST APIs, JPA, Oracle, and Angular 19."
    )
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Software Engineer",
        master_skills=["Java 21", "Spring Boot", "Angular 19"],
        master_summary="Java software engineer.",
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Development Engineer",
                bullets=[original],
                detected_skills=["Java", "Spring Boot", "Angular"],
            )
        ],
        required_skills=["Java", "Azure"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )
    draft = TailoringSuggestion(
        section="experience",
        original_text=original,
        suggested_text=(
            "Modernized Java 21 and Spring Boot services and deployed them to Azure "
            "App Service with managed configuration for cloud-ready healthcare workflows."
        ),
        requirements_addressed=["Azure"],
        skills_added=[],
        keywords_added=[],
        source="MASTER_RESUME",
        reason="Aligns the existing modernization work to the target cloud requirement.",
        confidence=0.7,
        risk_level="LOW",
    )

    sanitized = _sanitize_ai_tailoring(
        request,
        TailoringResponse(experience_suggestions=[draft]),
    )

    assert len(sanitized.experience_suggestions) == 1
    suggestion = sanitized.experience_suggestions[0]
    assert suggestion.source == "AI_SUGGESTED"
    assert suggestion.risk_level == "HIGH"
    assert "Azure" in suggestion.skills_added
    assert "candidate verification" in suggestion.reason.lower()
    assert "learning" not in suggestion.suggested_text.lower()
    assert "proficiency" not in suggestion.suggested_text.lower()


def test_ai_sanitizer_keeps_evidence_based_experience_rewrite() -> None:
    original = "Built Java Spring Boot REST APIs for payment services."
    request = TailoringRequest(
        mode="STRICT",
        job_title="Java Engineer",
        master_skills=["Java", "Spring Boot"],
        master_summary="Java software engineer.",
        experiences=[
            ExperienceInput(
                company="Acme",
                title="Software Engineer",
                bullets=[original],
                detected_skills=["Java", "Spring Boot", "REST"],
            )
        ],
        required_skills=["Java", "Spring Boot"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )
    suggestion = TailoringSuggestion(
        section="experience",
        original_text=original,
        suggested_text="Built Java Spring Boot REST APIs supporting payment services.",
        requirements_addressed=["Java", "Spring Boot"],
        source="MASTER_RESUME",
        reason="Tighter evidence-based wording.",
        confidence=0.9,
        risk_level="LOW",
    )

    sanitized = _sanitize_ai_tailoring(
        request,
        TailoringResponse(experience_suggestions=[suggestion]),
    )

    assert sanitized.experience_suggestions == [suggestion]
