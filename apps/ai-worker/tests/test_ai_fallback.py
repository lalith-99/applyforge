"""Tests for the AI-first-with-heuristic-fallback wiring in the route
handlers (no real network calls — the *_ai functions are monkeypatched)."""

from fastapi.testclient import TestClient

import app.api.jobs as jobs_module
import app.api.resumes as resumes_module
import app.api.tailoring as tailoring_module
from app.main import app
from app.providers.openai_provider import AIProviderError
from app.resume.models import ContactInfo, ResumeProfile

client = TestClient(app)

RAW_RESUME_TEXT = """Jordan Rivera
jordan.rivera@example.com | (555) 123-4567

SUMMARY
Backend engineer with 6 years of experience building distributed systems.

SKILLS
Go, Kafka, PostgreSQL
"""


def test_resume_parse_uses_ai_when_configured(monkeypatch) -> None:
    monkeypatch.setattr(resumes_module, "is_configured", lambda: True)
    monkeypatch.setattr(
        resumes_module,
        "parse_resume_text_ai",
        lambda raw_text: ResumeProfile(contact=ContactInfo(name="AI Parsed Name")),
    )

    response = client.post("/v1/resumes/parse", json={"raw_text": RAW_RESUME_TEXT})
    assert response.status_code == 200
    assert response.json()["profile"]["contact"]["name"] == "AI Parsed Name"


def test_resume_parse_falls_back_to_heuristic_on_ai_error(monkeypatch) -> None:
    monkeypatch.setattr(resumes_module, "is_configured", lambda: True)

    def failing_ai(raw_text: str) -> ResumeProfile:
        raise AIProviderError("boom")

    monkeypatch.setattr(resumes_module, "parse_resume_text_ai", failing_ai)

    response = client.post("/v1/resumes/parse", json={"raw_text": RAW_RESUME_TEXT})
    assert response.status_code == 200
    assert response.json()["profile"]["contact"]["name"] == "Jordan Rivera"


def test_resume_parse_uses_heuristic_when_not_configured(monkeypatch) -> None:
    monkeypatch.setattr(resumes_module, "is_configured", lambda: False)

    response = client.post("/v1/resumes/parse", json={"raw_text": RAW_RESUME_TEXT})
    assert response.status_code == 200
    assert response.json()["profile"]["contact"]["name"] == "Jordan Rivera"


def test_job_requirements_falls_back_to_heuristic_on_ai_error(monkeypatch) -> None:
    monkeypatch.setattr(jobs_module, "is_configured", lambda: True)

    def failing_ai(title: str, description: str):
        raise AIProviderError("boom")

    monkeypatch.setattr(jobs_module, "parse_job_requirements_ai", failing_ai)

    response = client.post(
        "/v1/jobs/parse-requirements",
        json={"title": "Backend Engineer", "description": "Required: Go, Kafka. Preferred: AWS."},
    )
    assert response.status_code == 200
    required = response.json()["requirements"]["required_skills"]
    normalized_names = {s["normalized_name"] for s in required}
    assert "Go" in normalized_names


def test_tailoring_suggest_falls_back_to_heuristic_on_ai_error(monkeypatch) -> None:
    monkeypatch.setattr(tailoring_module, "is_configured", lambda: True)

    def failing_ai(request):
        raise AIProviderError("boom")

    monkeypatch.setattr(tailoring_module, "generate_tailoring_ai", failing_ai)

    response = client.post(
        "/v1/tailoring/suggest",
        json={
            "mode": "STRICT",
            "job_title": "Senior Backend Engineer",
            "master_skills": ["Go"],
            "master_summary": "Backend engineer.",
            "experiences": [],
            "required_skills": ["Go"],
            "preferred_skills": [],
            "responsibilities": [],
            "transferable_matches": [],
        },
    )
    assert response.status_code == 200
