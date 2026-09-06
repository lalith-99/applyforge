"""Embeddings endpoint tests (no real network calls - the OpenAI client is
mocked at the provider layer)."""

from fastapi.testclient import TestClient

from app.api import embeddings as embeddings_module
from app.main import app

client = TestClient(app)


def test_embed_returns_503_when_not_configured(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    response = client.post("/v1/embeddings", json={"text": "backend engineer"})
    assert response.status_code == 503


def test_embed_returns_400_for_empty_text(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    response = client.post("/v1/embeddings", json={"text": "   "})
    assert response.status_code == 400


def test_embed_returns_vector_when_configured(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setattr(embeddings_module, "embed_text", lambda text: [0.1, 0.2, 0.3])

    response = client.post("/v1/embeddings", json={"text": "backend engineer"})
    assert response.status_code == 200
    body = response.json()
    assert body["embedding"] == [0.1, 0.2, 0.3]
    assert body["dimensions"] == 3
