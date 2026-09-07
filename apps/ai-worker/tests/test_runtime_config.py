from __future__ import annotations

import pytest

from app.main import validate_runtime_config


def test_production_requires_openai_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    with pytest.raises(RuntimeError, match="OPENAI_API_KEY"):
        validate_runtime_config()


def test_development_allows_heuristic_fallback(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ENVIRONMENT", "development")
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    validate_runtime_config()


def test_production_accepts_openai_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("OPENAI_API_KEY", "test-key")

    validate_runtime_config()
