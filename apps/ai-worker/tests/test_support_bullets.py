from app.tailoring.models import (
    ExperienceInput,
    TailoringRequest,
    TailoringResponse,
    TailoringSuggestion,
)
from app.tailoring.support_bullets import ensure_max_match_support_ai


def _skill_card(skill: str, category: str = "AI / GenAI") -> TailoringSuggestion:
    return TailoringSuggestion(
        section="skills",
        suggested_text=f"Add {skill}",
        requirements_addressed=[skill],
        skills_added=[skill],
        keywords_added=[skill],
        skill_categories={skill: category},
        source="AI_SUGGESTED",
        reason="Required by target role.",
        risk_level="MEDIUM",
    )


def _request() -> TailoringRequest:
    return TailoringRequest(
        mode="MAX_MATCH",
        job_title="Senior Software Engineer in Test",
        job_description="Build agentic AI test infrastructure and CI/CD automation.",
        master_skills=["Java", "Python", "Docker", "Kubernetes"],
        experiences=[
            ExperienceInput(
                company="CMS",
                title="Software Development Engineer",
                bullets=[
                    "Built an internal RAG search solution using LangChain and Bedrock.",
                    "Used Claude Code to support dependency upgrades and secure refactoring.",
                ],
                detected_skills=["Java", "RAG", "LangChain", "Bedrock", "Claude Code"],
            ),
            ExperienceInput(
                company="DXC Technology",
                title="Software Engineer",
                bullets=[
                    "Built and deployed Java services using Docker and Kubernetes.",
                    "Automated CI/CD workflows for enterprise application releases.",
                ],
                detected_skills=["Java", "Docker", "Kubernetes", "CI/CD"],
            ),
        ],
        required_skills=[
            "Prompt Engineering",
            "Tool Calling",
            "AI Agent Memory",
            "Agent Orchestration",
            "Agent Frameworks",
            "Model Context Protocol",
            "AI Observability",
            "Automated Testing Frameworks",
            "API Testing",
            "Data Pipeline Testing",
            "Infrastructure as Code",
            "Linux",
            "Monitoring",
        ],
    )


def _rewrite(original: str, text: str, skills: list[str]) -> TailoringSuggestion:
    return TailoringSuggestion(
        section="experience",
        original_text=original,
        suggested_text=text,
        requirements_addressed=skills,
        skills_added=skills,
        keywords_added=skills,
        operation="REWRITE",
        source="AI_SUGGESTED",
        reason="Supports a coherent target-skill cluster.",
        risk_level="HIGH",
    )


def test_max_match_recovers_three_support_bullets_for_large_skill_gap(monkeypatch) -> None:
    request = _request()
    result = TailoringResponse(
        skill_suggestions=[
            _skill_card("Prompt Engineering"),
            _skill_card("Tool Calling"),
            _skill_card("AI Agent Memory"),
            _skill_card("Agent Orchestration"),
            _skill_card("Agent Frameworks"),
            _skill_card("Model Context Protocol"),
            _skill_card("AI Observability"),
            _skill_card("Automated Testing Frameworks", "Testing & Tools"),
            _skill_card("API Testing", "Testing & Tools"),
            _skill_card("Data Pipeline Testing", "Testing & Tools"),
            _skill_card("Infrastructure as Code", "Cloud & DevOps"),
            _skill_card("Linux", "Cloud & DevOps"),
            _skill_card("Monitoring", "Cloud & DevOps"),
        ]
    )
    candidates = [
        _rewrite(
            request.experiences[0].bullets[0],
            "Built agentic retrieval workflows using prompt engineering, tool calling, and MCP.",
            ["Prompt Engineering", "Tool Calling", "Model Context Protocol"],
        ),
        _rewrite(
            request.experiences[1].bullets[0],
            "Built automated API and data-pipeline test workflows for containerized Java services.",
            ["Automated Testing Frameworks", "API Testing", "Data Pipeline Testing"],
        ),
        TailoringSuggestion(
            section="experience",
            suggested_text=(
                "Standardized Linux delivery environments with infrastructure-as-code workflows "
                "and service monitoring for repeatable deployments."
            ),
            requirements_addressed=["Infrastructure as Code", "Linux", "Monitoring"],
            skills_added=["Infrastructure as Code", "Linux", "Monitoring"],
            keywords_added=["Infrastructure as Code", "Linux", "Monitoring"],
            operation="ADD",
            target_company="DXC Technology",
            target_title="Software Engineer",
            source="AI_SUGGESTED",
            reason="Adds infrastructure support.",
            risk_level="HIGH",
        ),
    ]

    def fake_completion(system, user, response_model, **kwargs):
        return response_model(experience_suggestions=candidates)

    monkeypatch.setattr(
        "app.tailoring.support_bullets.structured_completion",
        fake_completion,
    )

    recovered = ensure_max_match_support_ai(request, result)

    assert len(recovered.experience_suggestions) == 3
    assert all(item.source == "AI_SUGGESTED" for item in recovered.experience_suggestions)
    assert all(item.risk_level == "HIGH" for item in recovered.experience_suggestions)
    assert recovered.experience_suggestions[2].operation == "ADD"
    assert recovered.experience_suggestions[2].target_company == "DXC Technology"


def test_support_recovery_does_not_run_when_enough_support_already_exists(
    monkeypatch,
) -> None:
    request = _request()
    result = TailoringResponse(
        skill_suggestions=[
            _skill_card("Prompt Engineering"),
            _skill_card("API Testing", "Testing & Tools"),
            _skill_card("Linux", "Cloud & DevOps"),
            _skill_card("Monitoring", "Cloud & DevOps"),
        ],
        experience_suggestions=[
            _rewrite(
                request.experiences[0].bullets[0],
                "Used prompt engineering for retrieval workflows.",
                ["Prompt Engineering"],
            ),
            _rewrite(
                request.experiences[1].bullets[0],
                "Validated APIs in Linux-hosted service environments.",
                ["API Testing", "Linux"],
            ),
        ],
    )

    def fail_if_called(*args, **kwargs):
        raise AssertionError("support recovery should not call the model")

    monkeypatch.setattr(
        "app.tailoring.support_bullets.structured_completion",
        fail_if_called,
    )

    recovered = ensure_max_match_support_ai(request, result)
    assert len(recovered.experience_suggestions) == 2


def test_support_recovery_retries_once_after_invalid_role_anchor(monkeypatch) -> None:
    request = _request()
    result = TailoringResponse(
        skill_suggestions=[
            _skill_card("Prompt Engineering"),
            _skill_card("API Testing", "Testing & Tools"),
            _skill_card("Linux", "Cloud & DevOps"),
            _skill_card("Monitoring", "Cloud & DevOps"),
        ]
    )
    valid_rewrite = _rewrite(
        request.experiences[0].bullets[0],
        "Built prompt-driven retrieval workflows for internal engineering search.",
        ["Prompt Engineering"],
    )
    invalid_add = TailoringSuggestion(
        section="experience",
        suggested_text="Built automated API tests.",
        requirements_addressed=["API Testing"],
        skills_added=["API Testing"],
        operation="ADD",
        target_company="Invented Co",
        target_title="Principal Engineer",
        source="AI_SUGGESTED",
        reason="Invalid role anchor.",
        risk_level="HIGH",
    )
    retry = _rewrite(
        request.experiences[1].bullets[0],
        "Built automated API validation for containerized services on Linux.",
        ["API Testing", "Linux"],
    )
    calls = 0

    def fake_completion(system, user, response_model, **kwargs):
        nonlocal calls
        calls += 1
        items = [valid_rewrite, invalid_add] if calls == 1 else [retry]
        return response_model(experience_suggestions=items)

    monkeypatch.setattr(
        "app.tailoring.support_bullets.structured_completion",
        fake_completion,
    )

    recovered = ensure_max_match_support_ai(request, result)

    assert calls == 2
    assert len(recovered.experience_suggestions) == 2
    assert all(item.target_company != "Invented Co" for item in recovered.experience_suggestions)
