"""OpenAI-backed structured completions for resume parsing, JD parsing, and
resume tailoring (see MASTER_REQUIREMENTS.md §45, DECISIONS.md Phases 2-7).

Every call site that uses this provider must catch `AIProviderError` and
fall back to the deterministic heuristic implementation — the API being
unset, rate-limited, or transiently down must never break the product.
"""

from __future__ import annotations

import os

from openai import OpenAI
from pydantic import BaseModel

_DEFAULT_MODEL = "gpt-4o-mini"
_DEFAULT_EMBEDDING_MODEL = "text-embedding-3-small"


class AIProviderError(RuntimeError):
    """Raised for any failure to obtain a real AI completion (missing key,
    network/API failure, or an empty/invalid response). Callers should catch
    this and fall back to the heuristic implementation."""


def is_configured() -> bool:
    return bool(os.environ.get("OPENAI_API_KEY"))


_client: OpenAI | None = None


def _get_client() -> OpenAI:
    global _client
    if _client is None:
        if not is_configured():
            raise AIProviderError("OPENAI_API_KEY is not set")
        _client = OpenAI()
    return _client


def structured_completion[T: BaseModel](
    system_prompt: str, user_prompt: str, response_model: type[T]
) -> T:
    """Requests a chat completion from OpenAI constrained to response_model's
    JSON schema, and returns it parsed + validated as that Pydantic model."""
    client = _get_client()
    model = os.environ.get("OPENAI_MODEL", _DEFAULT_MODEL)
    try:
        completion = client.chat.completions.parse(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            response_format=response_model,
            temperature=0.2,
        )
    except Exception as exc:  # openai raises several distinct exception types
        raise AIProviderError(f"OpenAI request failed: {exc}") from exc

    parsed = completion.choices[0].message.parsed
    if parsed is None:
        raise AIProviderError("OpenAI returned no parsed structured output")
    return parsed


def embed_text(text: str) -> list[float]:
    """Returns a semantic embedding vector for text (see MASTER_REQUIREMENTS.md
    Phase E: job/candidate embeddings for semantic retrieval via pgvector)."""
    client = _get_client()
    model = os.environ.get("OPENAI_EMBEDDING_MODEL", _DEFAULT_EMBEDDING_MODEL)
    try:
        response = client.embeddings.create(model=model, input=text)
    except Exception as exc:  # openai raises several distinct exception types
        raise AIProviderError(f"OpenAI embeddings request failed: {exc}") from exc

    if not response.data:
        raise AIProviderError("OpenAI returned no embedding data")
    return response.data[0].embedding
