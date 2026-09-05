"""Resume tailoring suggestion endpoint.

Called by the Go API when a user requests Tailor Resume for a job (see
apps/api/internal/tailoring). STRICT/GROWTH/MAX_MATCH mode logic lives in
app/tailoring/heuristics.py.
"""

from __future__ import annotations

from fastapi import APIRouter, HTTPException

from app.tailoring.heuristics import generate_tailoring
from app.tailoring.models import TAILORING_MODES, TailoringRequest, TailoringResponse

router = APIRouter(prefix="/v1/tailoring", tags=["tailoring"])


@router.post("/suggest", response_model=TailoringResponse)
def suggest(request: TailoringRequest) -> TailoringResponse:
    if request.mode not in TAILORING_MODES:
        raise HTTPException(status_code=422, detail=f"invalid mode: {request.mode}")
    return generate_tailoring(request)
