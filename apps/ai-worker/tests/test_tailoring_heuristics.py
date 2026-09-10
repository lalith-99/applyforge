"""Tests for heuristic resume tailoring suggestion generation."""

from app.tailoring.heuristics import (
    _sanitize_ai_tailoring,
    generate_tailoring,
    generate_tailoring_ai,
)
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




def test_summary_kafka_alias_is_not_misclassified_as_new_skill() -> None:
    original_summary = (
        "Java engineer with Spring Boot, Apache Kafka, AWS, Docker, and Kubernetes experience."
    )
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Staff Software Engineer",
        master_skills=["Java", "Spring Boot", "Apache Kafka", "AWS", "Docker", "Kubernetes"],
        master_summary=original_summary,
        experiences=[],
        required_skills=["Java", "Kafka"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )
    summary = TailoringSuggestion(
        section="summary",
        original_text=original_summary,
        suggested_text=(
            "Java engineer building low-latency services with Spring Boot, Kafka, AWS, "
            "Docker, and Kubernetes."
        ),
        requirements_addressed=["Java", "Kafka"],
        skills_added=["Kafka"],
        keywords_added=["Kafka"],
        source="AI_SUGGESTED",
        reason="Foregrounds relevant backend experience.",
        risk_level="HIGH",
    )

    classified = _sanitize_ai_tailoring(
        request,
        TailoringResponse(summary_suggestion=summary),
    )

    assert classified.summary_suggestion is not None
    assert classified.summary_suggestion.source == "MASTER_RESUME"
    assert classified.summary_suggestion.risk_level == "LOW"
    assert classified.summary_suggestion.skills_added == []

def test_ai_classifier_keeps_learning_style_draft_for_critic_review() -> None:
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

    assert len(sanitized.experience_suggestions) == 1
    suggestion = sanitized.experience_suggestions[0]
    assert suggestion.source == "AI_SUGGESTED"
    assert suggestion.risk_level == "HIGH"
    assert "Azure" in suggestion.skills_added




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


def test_max_match_ai_uses_one_pass_and_keeps_multiple_experience_rewrites(
    monkeypatch,
) -> None:
    first_bullet = "Built Java Spring Boot REST APIs for healthcare workflows."
    second_bullet = "Built Angular interfaces integrated with backend APIs."
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Mobile Platform Engineer",
        master_skills=["Java", "Spring Boot", "Angular"],
        master_summary="Java software engineer.",
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Development Engineer",
                bullets=[first_bullet, second_bullet],
                detected_skills=["Java", "Spring Boot", "Angular"],
            )
        ],
        required_skills=["Java", "Kotlin"],
        preferred_skills=["Swift"],
        responsibilities=["Build mobile-facing application services and client integrations."],
        transferable_matches=[],
    )

    initial = TailoringResponse(
        skill_suggestions=[
            TailoringSuggestion(
                section="skills",
                suggested_text="Add Kotlin to your skills section",
                requirements_addressed=["Kotlin"],
                skills_added=["Kotlin"],
                keywords_added=["Kotlin"],
                source="AI_SUGGESTED",
                reason="Required by the role.",
                risk_level="MEDIUM",
            ),
            TailoringSuggestion(
                section="skills",
                suggested_text="Add Swift to your skills section",
                requirements_addressed=["Swift"],
                skills_added=["Swift"],
                keywords_added=["Swift"],
                source="AI_SUGGESTED",
                reason="Preferred by the role.",
                risk_level="MEDIUM",
            ),
        ],
        experience_suggestions=[
            TailoringSuggestion(
                section="experience",
                original_text=first_bullet,
                suggested_text=(
                    "Developed Kotlin services alongside Java Spring Boot APIs to support "
                    "mobile-facing healthcare workflows and shared backend integrations."
                ),
                requirements_addressed=["Kotlin"],
                skills_added=["Kotlin"],
                keywords_added=["Kotlin"],
                source="AI_SUGGESTED",
                reason="Aligns backend service work to the target mobile platform stack.",
                risk_level="HIGH",
            ),
            TailoringSuggestion(
                section="experience",
                original_text=second_bullet,
                suggested_text=(
                    "Integrated Swift client workflows with REST APIs and Angular-backed "
                    "administrative services to support consistent mobile and web experiences."
                ),
                requirements_addressed=["Swift"],
                skills_added=["Swift"],
                keywords_added=["Swift"],
                source="AI_SUGGESTED",
                reason="Aligns client-facing work to the preferred mobile technology.",
                risk_level="HIGH",
            ),
        ],
        keyword_coverage_before=1 / 3,
        keyword_coverage_after=1.0,
    )

    calls = []

    def fake_structured_completion(
        system_prompt,
        user_prompt,
        response_model,
        *,
        model_env_var=None,
    ):
        calls.append(response_model)
        assert "at least 3 distinct experience rewrites" in system_prompt
        return initial

    monkeypatch.setattr(
        "app.providers.openai_provider.structured_completion",
        fake_structured_completion,
    )

    result = generate_tailoring_ai(request)

    assert calls == [TailoringResponse]
    assert {s.skills_added[0] for s in result.skill_suggestions} == {"Kotlin", "Swift"}
    assert len(result.experience_suggestions) == 2
    assert all(s.source == "AI_SUGGESTED" for s in result.experience_suggestions)
    assert all(s.risk_level == "HIGH" for s in result.experience_suggestions)


def test_ai_classifier_keeps_richer_duplicate_source_rewrite() -> None:
    original = "Built Java Spring Boot REST APIs for healthcare workflows."
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Platform Engineer",
        master_skills=["Java", "Spring Boot"],
        master_summary="Java software engineer.",
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Engineer",
                bullets=[original],
                detected_skills=["Java", "Spring Boot"],
            )
        ],
        required_skills=["Java", "Kubernetes", "Linux"],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )
    result = TailoringResponse(
        experience_suggestions=[
            TailoringSuggestion(
                section="experience",
                original_text=original,
                suggested_text="Built Java Spring Boot REST APIs for healthcare workflows.",
                requirements_addressed=["Java"],
                source="MASTER_RESUME",
                reason="Basic rewrite.",
            ),
            TailoringSuggestion(
                section="experience",
                original_text=original,
                suggested_text=(
                    "Built and operated Java Spring Boot services on Kubernetes and Linux "
                    "for resilient healthcare workflows."
                ),
                requirements_addressed=["Java", "Kubernetes", "Linux"],
                skills_added=["Kubernetes", "Linux"],
                source="AI_SUGGESTED",
                reason="Stronger platform alignment.",
                risk_level="HIGH",
            ),
        ]
    )

    classified = _sanitize_ai_tailoring(request, result)

    assert len(classified.experience_suggestions) == 1
    assert "Kubernetes" in classified.experience_suggestions[0].suggested_text
    assert "Linux" in classified.experience_suggestions[0].suggested_text

