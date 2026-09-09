"""Liveness and readiness endpoints.

The AI worker has no required external dependencies at startup (Phase 0), so
readiness currently mirrors liveness. This will be extended once the worker
depends on the object storage client or an AI provider connection.
"""

from fastapi import APIRouter

from app.providers.openai_provider import is_configured

router = APIRouter()


@router.get("/health")
def health() -> dict[str, str | bool]:
    return {"status": "ok", "openai_configured": is_configured()}


@router.get("/ready")
def ready() -> dict[str, str | bool]:
    return {"status": "ready", "openai_configured": is_configured()}
