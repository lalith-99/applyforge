"""OpenAI-backed structured completions for resume parsing, JD parsing, and
resume tailoring (see MASTER_REQUIREMENTS.md §45, DECISIONS.md Phases 2-7).

Every call site that uses this provider must catch `AIProviderError` and
fall back to the deterministic heuristic implementation — the API being
unset, rate-limited, or transiently down must never break the product.
"""

from __future__ import annotations

import os
import threading
from dataclasses import dataclass

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


@dataclass
class AIUsageMetadata:
    provider: str
    model: str
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    estimated_cost_usd: float | None = None


_usage_state = threading.local()


def clear_usage_metadata() -> None:
    _usage_state.value = None


def pop_usage_metadata() -> AIUsageMetadata | None:
    value = getattr(_usage_state, "value", None)
    _usage_state.value = None
    return value


def apply_usage_headers(response) -> None:
    usage = pop_usage_metadata()
    if usage is None:
        return
    response.headers["X-ApplyForge-AI-Provider"] = usage.provider
    response.headers["X-ApplyForge-AI-Model"] = usage.model
    response.headers["X-ApplyForge-AI-Prompt-Tokens"] = str(usage.prompt_tokens)
    response.headers["X-ApplyForge-AI-Completion-Tokens"] = str(usage.completion_tokens)
    response.headers["X-ApplyForge-AI-Total-Tokens"] = str(usage.total_tokens)
    if usage.estimated_cost_usd is not None:
        response.headers["X-ApplyForge-AI-Estimated-Cost-USD"] = f"{usage.estimated_cost_usd:.10f}"


def _env_float(name: str) -> float | None:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return None
    try:
        value = float(raw)
    except ValueError:
        return None
    return value if value >= 0 else None


def _record_chat_usage(model: str, usage) -> None:
    prompt = int(getattr(usage, "prompt_tokens", 0) or 0)
    completion = int(getattr(usage, "completion_tokens", 0) or 0)
    total = int(getattr(usage, "total_tokens", prompt + completion) or 0)
    input_rate = _env_float("OPENAI_INPUT_USD_PER_1M_TOKENS")
    output_rate = _env_float("OPENAI_OUTPUT_USD_PER_1M_TOKENS")
    estimated = None
    if input_rate is not None and output_rate is not None:
        estimated = (prompt * input_rate + completion * output_rate) / 1_000_000
    _usage_state.value = AIUsageMetadata(
        provider="openai",
        model=model,
        prompt_tokens=prompt,
        completion_tokens=completion,
        total_tokens=total,
        estimated_cost_usd=estimated,
    )


def _record_embedding_usage(model: str, usage) -> None:
    prompt = int(getattr(usage, "prompt_tokens", 0) or 0)
    total = int(getattr(usage, "total_tokens", prompt) or 0)
    rate = _env_float("OPENAI_EMBEDDING_USD_PER_1M_TOKENS")
    estimated = None if rate is None else total * rate / 1_000_000
    _usage_state.value = AIUsageMetadata(
        provider="openai",
        model=model,
        prompt_tokens=prompt,
        total_tokens=total,
        estimated_cost_usd=estimated,
    )


_client: OpenAI | None = None


def _get_client() -> OpenAI:
    global _client
    if _client is None:
        if not is_configured():
            raise AIProviderError("OPENAI_API_KEY is not set")
        _client = OpenAI()
    return _client


def structured_completion[T: BaseModel](
    system_prompt: str,
    user_prompt: str,
    response_model: type[T],
    *,
    model_env_var: str | None = None,
) -> T:
    """Requests a structured completion and validates it as response_model.

    model_env_var allows high-value/low-frequency operations such as resume
    parsing or tailoring to use a stronger model without forcing high-volume
    ranking/classification calls onto the same expensive model.

    Do not send custom sampling parameters here. GPT-5.x reasoning models can
    reject non-default temperature/top_p values, which would make the product
    silently fall back to heuristics instead of using the configured model.
    """
    client = _get_client()
    model = (
        os.environ.get(model_env_var, "").strip()
        if model_env_var
        else ""
    ) or os.environ.get("OPENAI_MODEL", _DEFAULT_MODEL)
    try:
        completion = client.chat.completions.parse(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            response_format=response_model,
        )
    except Exception as exc:  # openai raises several distinct exception types
        raise AIProviderError(f"OpenAI request failed: {exc}") from exc

    _record_chat_usage(model, getattr(completion, "usage", None))
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
    _record_embedding_usage(model, getattr(response, "usage", None))
    return response.data[0].embedding
