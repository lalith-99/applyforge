from app.tailoring.heuristics import generate_tailoring_ai
from app.tailoring.models import TailoringRequest, TailoringResponse, TailoringSuggestion


def test_ai_added_golang_display_is_canonicalized_to_go() -> None:
    suggestion = TailoringSuggestion(
        section="skills",
        suggested_text="Add Go (Golang) to your skills section",
        requirements_addressed=["Go"],
        skills_added=["Go (Golang)"],
        keywords_added=["Golang"],
        source="AI_SUGGESTED",
        reason="The role explicitly names Go.",
        risk_level="HIGH",
    )

    assert suggestion.skills_added == ["Go"]
    assert suggestion.keywords_added == ["Go"]


def test_tailoring_ai_receives_authoritative_full_job_description(monkeypatch) -> None:
    description = (
        "Design and build large-scale privacy infrastructure. "
        "Expertise in Python or Golang. Develop reliable systems for safe data access."
    )
    request = TailoringRequest(
        mode="MAX_MATCH",
        job_title="Senior Software Engineer, Privacy Infrastructure",
        job_description=description,
        master_skills=["Python", "AWS", "Kubernetes"],
        master_summary="Senior software engineer building distributed systems.",
        experiences=[],
        required_skills=["Python"],
        preferred_skills=[],
        responsibilities=["Design and build large-scale privacy infrastructure."],
        transferable_matches=[],
    )

    def fake_structured_completion(
        system_prompt,
        user_prompt,
        response_model,
        *,
        model_env_var=None,
    ):
        assert response_model is TailoringResponse
        assert description in user_prompt
        assert '"job_description"' in user_prompt
        assert "resume-wide tailoring pass" in system_prompt
        return TailoringResponse()

    monkeypatch.setattr(
        "app.providers.openai_provider.structured_completion",
        fake_structured_completion,
    )

    result = generate_tailoring_ai(request)
    assert result == TailoringResponse(keyword_coverage_before=1.0, keyword_coverage_after=1.0)
