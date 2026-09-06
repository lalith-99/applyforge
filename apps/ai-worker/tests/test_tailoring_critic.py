"""Tailoring critique endpoint tests (no real network calls)."""

from fastapi.testclient import TestClient

from app.api import tailoring as tailoring_module
from app.main import app
from app.tailoring.critic_models import CritiqueResult

client = TestClient(app)


def test_critique_falls_back_to_heuristic_when_not_configured(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    response = client.post(
        "/v1/tailoring/critique",
        json={
            "job_title": "Backend Engineer",
            "required_skills": ["Go", "Kafka"],
            "suggestions": [
                {"section": "skills", "suggested_text": "Added Go", "skills_added": ["Go"]}
            ],
        },
    )
    assert response.status_code == 200
    result = response.json()["result"]
    assert "Kafka" in result["missing_high_value_keywords"]
    assert result["recommend_regeneration"] is False


def test_critique_uses_ai_when_configured(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    fake_result = CritiqueResult(
        unsupported_claims=["fabricated AWS cert"], ats_score=60, recommend_regeneration=True
    )
    monkeypatch.setattr(tailoring_module, "critique_ai", lambda req: fake_result)

    response = client.post("/v1/tailoring/critique", json={"job_title": "Backend Engineer"})
    assert response.status_code == 200
    result = response.json()["result"]
    assert result["recommend_regeneration"] is True
    assert result["unsupported_claims"] == ["fabricated AWS cert"]


def test_critique_falls_back_on_ai_error(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    def _raise(req):
        from app.providers.openai_provider import AIProviderError

        raise AIProviderError("boom")

    monkeypatch.setattr(tailoring_module, "critique_ai", _raise)

    response = client.post("/v1/tailoring/critique", json={"job_title": "Backend Engineer"})
    assert response.status_code == 200
    assert response.json()["result"]["recommend_regeneration"] is False
