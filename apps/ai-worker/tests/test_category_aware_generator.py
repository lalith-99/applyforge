import fitz

from app.documents.category_aware_generator import _enhanced_fallback, render_pdf
from app.resume.models import ContactInfo, ResumeProfile


def test_modern_ai_and_testing_skills_have_sensible_fallback_categories() -> None:
    assert _enhanced_fallback("Prompt Engineering") == "AI / GenAI"
    assert _enhanced_fallback("Model Context Protocol") == "AI / GenAI"
    assert _enhanced_fallback("Agent Orchestration") == "AI / GenAI"
    assert _enhanced_fallback("Cursor") == "AI / GenAI"
    assert _enhanced_fallback("Automated Testing Frameworks") == "Testing & Tools"
    assert _enhanced_fallback("API Testing") == "Testing & Tools"
    assert _enhanced_fallback("Data Pipeline Testing") == "Testing & Tools"
    assert _enhanced_fallback("Linux") == "Cloud & DevOps"
    assert _enhanced_fallback("Monitoring") == "Cloud & DevOps"


def test_pdf_honors_explicit_ai_skill_category_overrides() -> None:
    profile = ResumeProfile(
        contact=ContactInfo(name="Ada Lovelace"),
        skills=["Prompt Engineering", "Model Context Protocol", "Linux"],
        skill_categories={
            "Prompt Engineering": "AI / GenAI",
            "Model Context Protocol": "AI / GenAI",
            "Linux": "Cloud & DevOps",
        },
    )

    data = render_pdf(profile)
    with fitz.open(stream=data, filetype="pdf") as doc:
        text = "\n".join(page.get_text() for page in doc)

    assert "AI / GenAI:" in text
    assert "Prompt Engineering" in text
    assert "Model Context Protocol" in text
    assert "Cloud & DevOps:" in text
    assert "Linux" in text
    assert "Other:" not in text


def test_explicit_ai_category_can_override_deterministic_fallback() -> None:
    profile = ResumeProfile(
        contact=ContactInfo(name="Ada Lovelace"),
        skills=["Monitoring"],
        # The deterministic fallback would choose Cloud & DevOps, but the
        # model can choose AI / GenAI when the JD specifically means AI-system
        # monitoring/evaluation.
        skill_categories={"Monitoring": "AI / GenAI"},
    )

    data = render_pdf(profile)
    with fitz.open(stream=data, filetype="pdf") as doc:
        text = "\n".join(page.get_text() for page in doc)

    assert "AI / GenAI:" in text
    assert "Monitoring" in text
    assert "Cloud & DevOps:" not in text
