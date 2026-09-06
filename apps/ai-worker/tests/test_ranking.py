"""Job ranking endpoint tests (no real network calls)."""

from fastapi.testclient import TestClient

from app.api import candidates as candidates_module
from app.candidates.ranking_models import JobRankingResult, RankJobsResult
from app.main import app

client = TestClient(app)


def test_rank_jobs_returns_empty_for_no_jobs(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    response = client.post("/v1/candidates/rank-jobs", json={"jobs": []})
    assert response.status_code == 200
    assert response.json()["result"]["rankings"] == []


def test_rank_jobs_falls_back_to_heuristic_when_not_configured(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    response = client.post(
        "/v1/candidates/rank-jobs",
        json={
            "jobs": [
                {
                    "job_id": "job-1",
                    "title": "Backend Engineer",
                    "company_name": "Acme",
                    "matched_skills": ["Go", "Kafka"],
                    "missing_required_skills": [],
                    "deterministic_score": 90,
                }
            ]
        },
    )
    assert response.status_code == 200
    rankings = response.json()["result"]["rankings"]
    assert len(rankings) == 1
    assert rankings[0]["job_id"] == "job-1"
    assert rankings[0]["recommendation"] == "APPLY_NOW"


def test_rank_jobs_uses_ai_when_configured(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    fake_result = RankJobsResult(
        rankings=[JobRankingResult(job_id="job-1", fit_score=95, recommendation="APPLY_NOW")]
    )
    monkeypatch.setattr(candidates_module, "rank_jobs_ai", lambda req: fake_result)

    response = client.post(
        "/v1/candidates/rank-jobs",
        json={"jobs": [{"job_id": "job-1", "title": "Backend Engineer", "company_name": "Acme"}]},
    )
    assert response.status_code == 200
    assert response.json()["result"]["rankings"][0]["fit_score"] == 95


def test_rank_jobs_falls_back_on_ai_error(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    def _raise(req):
        from app.providers.openai_provider import AIProviderError

        raise AIProviderError("boom")

    monkeypatch.setattr(candidates_module, "rank_jobs_ai", _raise)

    response = client.post(
        "/v1/candidates/rank-jobs",
        json={
            "jobs": [
                {
                    "job_id": "job-1",
                    "title": "Backend Engineer",
                    "company_name": "Acme",
                    "deterministic_score": 40,
                }
            ]
        },
    )
    assert response.status_code == 200
    assert response.json()["result"]["rankings"][0]["recommendation"] == "SKIP"
