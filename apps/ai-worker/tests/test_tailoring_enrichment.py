from app.tailoring.enrichment import (
    TailoringEnrichmentResponse,
    enrich_tailoring_ai,
)
from app.tailoring.models import (
    ExperienceInput,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
)


def _request() -> TailoringRequest:
    return TailoringRequest(
        mode="MAX_MATCH",
        job_title="Senior Software Engineer in Test",
        job_description=(
            "Build agentic AI test infrastructure using prompt engineering, MCP, "
            "tool calling, Linux, CI/CD, and automated API/data-pipeline testing."
        ),
        master_skills=["Java", "Python", "Docker", "Kubernetes"],
        master_summary="Software engineer building enterprise services.",
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Development Engineer",
                bullets=[
                    (
                        "Built an internal RAG-based search solution using LangChain "
                        "and Amazon Bedrock."
                    ),
                    "Used Claude Code to support dependency upgrades and secure refactoring.",
                ],
                detected_skills=[
                    "Java",
                    "RAG",
                    "LangChain",
                    "Amazon Bedrock",
                    "Claude Code",
                ],
            )
        ],
        required_skills=[
            "Prompt Engineering",
            "Model Context Protocol",
            "Automated Testing Frameworks",
            "Linux",
        ],
        preferred_skills=[],
        responsibilities=[],
        transferable_matches=[],
    )


def _skill_suggestion(skill: str) -> TailoringSuggestion:
    return TailoringSuggestion(
        section="skills",
        suggested_text=f"Add {skill}",
        requirements_addressed=[skill],
        skills_added=[skill],
        keywords_added=[skill],
        source="AI_SUGGESTED",
        reason="Required by target role.",
        risk_level="MEDIUM",
    )


def test_enrichment_uses_ai_selected_resume_categories(monkeypatch) -> None:
    request = _request()
    result = TailoringResponse(
        skill_suggestions=[
            _skill_suggestion("Prompt Engineering"),
            _skill_suggestion("Model Context Protocol"),
            _skill_suggestion("Automated Testing Frameworks"),
            _skill_suggestion("Linux"),
        ]
    )

    def fake_structured_completion(*args, **kwargs):
        return TailoringEnrichmentResponse(
            skill_categories={
                "Prompt Engineering": "AI / GenAI",
                "Model Context Protocol": "AI / GenAI",
                "Automated Testing Frameworks": "Testing & Tools",
                "Linux": "Cloud & DevOps",
            }
        )

    monkeypatch.setattr(
        "app.tailoring.enrichment.structured_completion",
        fake_structured_completion,
    )

    enriched = enrich_tailoring_ai(request, result)
    categories = {
        skill: suggestion.skill_categories[skill]
        for suggestion in enriched.skill_suggestions
        for skill in suggestion.skills_added
    }

    assert categories == {
        "Prompt Engineering": "AI / GenAI",
        "Model Context Protocol": "AI / GenAI",
        "Automated Testing Frameworks": "Testing & Tools",
        "Linux": "Cloud & DevOps",
    }


def test_enrichment_can_add_one_attestation_gated_bullet_to_existing_role(
    monkeypatch,
) -> None:
    request = _request()
    result = TailoringResponse(
        skill_suggestions=[_skill_suggestion("Prompt Engineering")]
    )
    add = TailoringSuggestion(
        section="experience",
        original_text=None,
        suggested_text=(
            "Built agentic test orchestration workflows using prompt engineering and "
            "tool calling to automate repeatable integration-test execution and triage."
        ),
        requirements_addressed=["agentic AI workflows", "Prompt Engineering"],
        skills_added=["Prompt Engineering", "Tool Calling"],
        keywords_added=["Prompt Engineering", "Tool Calling"],
        skill_categories={
            "Prompt Engineering": "AI / GenAI",
            "Tool Calling": "AI / GenAI",
        },
        operation="ADD",
        target_company="CMS",
        target_title="Software Development Engineer",
        source="AI_SUGGESTED",
        reason=(
            "Adds missing agentic-testing evidence to the most compatible AI-enabled role."
        ),
        risk_level="HIGH",
    )

    monkeypatch.setattr(
        "app.tailoring.enrichment.structured_completion",
        lambda *args, **kwargs: TailoringEnrichmentResponse(
            skill_categories={"Prompt Engineering": "AI / GenAI"},
            experience_suggestions=[add],
        ),
    )

    enriched = enrich_tailoring_ai(request, result)

    assert len(enriched.experience_suggestions) == 1
    suggestion = enriched.experience_suggestions[0]
    assert suggestion.operation == "ADD"
    assert suggestion.original_text is None
    assert suggestion.target_company == "CMS"
    assert suggestion.target_title == "Software Development Engineer"
    assert suggestion.source == "AI_SUGGESTED"
    assert suggestion.risk_level == "HIGH"


def test_enrichment_drops_new_bullet_targeting_nonexistent_role(monkeypatch) -> None:
    request = _request()
    invalid = TailoringSuggestion(
        section="experience",
        suggested_text="Built an MCP server for automated testing.",
        requirements_addressed=["MCP"],
        skills_added=["Model Context Protocol"],
        operation="ADD",
        target_company="Invented Company",
        target_title="Principal AI Engineer",
        source="AI_SUGGESTED",
        reason="Invalid target should be discarded.",
        risk_level="HIGH",
    )

    monkeypatch.setattr(
        "app.tailoring.enrichment.structured_completion",
        lambda *args, **kwargs: TailoringEnrichmentResponse(
            experience_suggestions=[invalid]
        ),
    )

    enriched = enrich_tailoring_ai(request, TailoringResponse())
    assert enriched.experience_suggestions == []
