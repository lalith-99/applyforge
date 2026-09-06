"""Resume tailoring suggestion endpoint.

Called by the Go API when a user requests Tailor Resume for a job (see
apps/api/internal/tailoring). STRICT/GROWTH/MAX_MATCH mode logic lives in
app/tailoring/heuristics.py.
"""

from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException

from app.providers.openai_provider import AIProviderError, is_configured
from app.tailoring.critic import critique_ai, critique_heuristic
from app.tailoring.critic_models import CritiqueRequest, CritiqueResponse
from app.tailoring.heuristics import generate_tailoring, generate_tailoring_ai
from app.tailoring.models import TAILORING_MODES, TailoringRequest, TailoringResponse

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/tailoring", tags=["tailoring"])


@router.post("/suggest", response_model=TailoringResponse)
def suggest(request: TailoringRequest) -> TailoringResponse:
    if request.mode not in TAILORING_MODES:
        raise HTTPException(status_code=422, detail=f"invalid mode: {request.mode}")

    if is_configured():
        try:
            return generate_tailoring_ai(request)
        except AIProviderError:
            logger.warning(
                "AI tailoring generation failed, falling back to heuristic", exc_info=True
            )

    return generate_tailoring(request)


@router.post("/critique", response_model=CritiqueResponse)
def critique(request: CritiqueRequest) -> CritiqueResponse:
    if is_configured():
        try:
            return CritiqueResponse(result=critique_ai(request))
        except AIProviderError:
            logger.warning("AI tailoring critique failed, falling back to heuristic", exc_info=True)

    return CritiqueResponse(result=critique_heuristic(request))
