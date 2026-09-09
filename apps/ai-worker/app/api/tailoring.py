"""Resume tailoring suggestion endpoint.

Called by the Go API when a user requests Tailor Resume for a job (see
apps/api/internal/tailoring). STRICT/GROWTH/MAX_MATCH mode logic lives in
app/tailoring/heuristics.py.
"""

from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException, Response

from app.providers.openai_provider import (
    AIProviderError,
    apply_usage_headers,
    clear_usage_metadata,
    is_configured,
)
from app.tailoring.critic import critique_ai, critique_heuristic
from app.tailoring.critic_models import CritiqueRequest, CritiqueResponse
from app.tailoring.heuristics import generate_tailoring, generate_tailoring_ai
from app.tailoring.models import TAILORING_MODES, TailoringRequest, TailoringResponse

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/tailoring", tags=["tailoring"])


@router.post("/suggest", response_model=TailoringResponse)
def suggest(request: TailoringRequest, response: Response) -> TailoringResponse:
    if request.mode not in TAILORING_MODES:
        raise HTTPException(status_code=422, detail=f"invalid mode: {request.mode}")

    if is_configured():
        clear_usage_metadata()
        try:
            result = generate_tailoring_ai(request)
            apply_usage_headers(response)
            return result
        except AIProviderError:
            logger.warning(
                "AI tailoring generation failed, falling back to heuristic", exc_info=True
            )

    fallback = generate_tailoring(request)
    # When the AI provider is unavailable, keep the master resume prose intact.
    # The deterministic fallback is useful for surfacing missing skill keywords,
    # but its summary/experience rewrites read like match-analysis metadata
    # ("directly applicable to this role") rather than polished resume copy.
    fallback.summary_suggestion = None
    fallback.experience_suggestions = []
    return fallback


@router.post("/critique", response_model=CritiqueResponse)
def critique(request: CritiqueRequest, response: Response) -> CritiqueResponse:
    if is_configured():
        clear_usage_metadata()
        try:
            result = CritiqueResponse(result=critique_ai(request))
            apply_usage_headers(response)
            return result
        except AIProviderError:
            logger.warning("AI tailoring critique failed, falling back to heuristic", exc_info=True)

    return CritiqueResponse(result=critique_heuristic(request))
