"""Tests for the OpenAI-backed structured completion provider (no real
network calls — the SDK client is mocked)."""

import pytest
from pydantic import BaseModel

from app.providers import openai_provider


class _Dummy(BaseModel):
    value: str


@pytest.fixture(autouse=True)
def _reset_client_cache():
    openai_provider._client = None
    yield
    openai_provider._client = None


def test_is_configured_reflects_env_var(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    assert openai_provider.is_configured() is False
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    assert openai_provider.is_configured() is True


def test_structured_completion_raises_when_not_configured(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    with pytest.raises(openai_provider.AIProviderError):
        openai_provider.structured_completion("system", "user", _Dummy)


def test_structured_completion_returns_parsed_model(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    class _FakeCompletions:
        def parse(self, **kwargs):
            assert kwargs["response_format"] is _Dummy
            message = type("M", (), {"parsed": _Dummy(value="hello")})()
            choice = type("C", (), {"message": message})()
            return type("Completion", (), {"choices": [choice]})()

    fake_chat = type("Chat", (), {"completions": _FakeCompletions()})()
    fake_client = type("Client", (), {"chat": fake_chat})()
    monkeypatch.setattr(openai_provider, "_get_client", lambda: fake_client)

    result = openai_provider.structured_completion("system", "user", _Dummy)
    assert result == _Dummy(value="hello")


def test_structured_completion_wraps_client_errors(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    class _FailingCompletions:
        def parse(self, **kwargs):
            raise RuntimeError("boom")

    fake_client = type(
        "Client", (), {"chat": type("Chat", (), {"completions": _FailingCompletions()})()}
    )()
    monkeypatch.setattr(openai_provider, "_get_client", lambda: fake_client)

    with pytest.raises(openai_provider.AIProviderError):
        openai_provider.structured_completion("system", "user", _Dummy)


def test_structured_completion_raises_when_parsed_is_none(monkeypatch) -> None:
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")

    class _FakeCompletions:
        def parse(self, **kwargs):
            message = type("M", (), {"parsed": None})()
            choice = type("C", (), {"message": message})()
            return type("Completion", (), {"choices": [choice]})()

    fake_chat = type("Chat", (), {"completions": _FakeCompletions()})()
    fake_client = type("Client", (), {"chat": fake_chat})()
    monkeypatch.setattr(openai_provider, "_get_client", lambda: fake_client)

    with pytest.raises(openai_provider.AIProviderError):
        openai_provider.structured_completion("system", "user", _Dummy)
