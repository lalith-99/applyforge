"""Candidate intelligence profile endpoint (Phase F)."""

from __future__ import annotations

import logging

from fastapi import APIRouter

from app.candidates.models import CandidateProfileRequest, CandidateProfileResponse
from app.candidates.synthesis import synthesize_profile_ai, synthesize_profile_heuristic
from app.providers.openai_provider import AIProviderError, is_configured

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/candidates", tags=["candidates"])


@router.post("/profile", response_model=CandidateProfileResponse)
def build_profile(request: CandidateProfileRequest) -> CandidateProfileResponse:
    if is_configured():
        try:
            profile = synthesize_profile_ai(request)
            return CandidateProfileResponse(profile=profile)
        except AIProviderError:
            logger.warning(
                "AI candidate profile synthesis failed, falling back to heuristic", exc_info=True
            )

    profile = synthesize_profile_heuristic(request)
    return CandidateProfileResponse(profile=profile)
