"""Candidate profile endpoint tests (no real network calls)."""

from fastapi.testclient import TestClient

from app.api import candidates as candidates_module
from app.candidates.models import CandidateIntelligenceProfile
from app.main import app

client = TestClient(app)


def test_build_profile_falls_back_to_heuristic_when_not_configured(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    response = client.post(
        "/v1/candidates/profile",
        json={
            "target_roles": ["Backend Engineer"],
            "seniority": "Senior",
            "master_skills": ["Go", "PostgreSQL"],
            "experiences": [
                {
                    "company": "Acme",
                    "title": "Senior Backend Engineer",
                    "bullets": ["Built a payments service handling 10k req/s"],
                    "technologies": ["Kafka", "Kubernetes"],
                }
            ],
        },
    )
    assert response.status_code == 200
    body = response.json()["profile"]
    assert body["core_skills"] == ["Go", "PostgreSQL"]
    assert "Kafka" in body["secondary_skills"]
    assert body["target_roles"] == ["Backend Engineer"]


def test_build_profile_uses_ai_when_configured(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    fake_profile = CandidateIntelligenceProfile(
        target_roles=["Backend Engineer"],
        core_skills=["Go"],
        summary="Senior backend engineer targeting distributed systems roles.",
    )
    monkeypatch.setattr(candidates_module, "synthesize_profile_ai", lambda req: fake_profile)

    response = client.post("/v1/candidates/profile", json={"target_roles": ["Backend Engineer"]})
    assert response.status_code == 200
    assert response.json()["profile"]["summary"] == fake_profile.summary


def test_build_profile_falls_back_on_ai_error(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    def _raise(req):
        from app.providers.openai_provider import AIProviderError

        raise AIProviderError("boom")

    monkeypatch.setattr(candidates_module, "synthesize_profile_ai", _raise)

    response = client.post(
        "/v1/candidates/profile",
        json={"target_roles": ["Backend Engineer"], "master_skills": ["Go"]},
    )
    assert response.status_code == 200
    assert response.json()["profile"]["core_skills"] == ["Go"]
