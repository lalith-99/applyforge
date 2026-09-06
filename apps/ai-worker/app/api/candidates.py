"""Candidate intelligence profile and job-fit ranking endpoints (Phases F/H)."""

from __future__ import annotations

import logging

from fastapi import APIRouter

from app.candidates.models import CandidateProfileRequest, CandidateProfileResponse
from app.candidates.ranking import rank_jobs_ai, rank_jobs_heuristic
from app.candidates.ranking_models import RankJobsRequest, RankJobsResponse
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


@router.post("/rank-jobs", response_model=RankJobsResponse)
def rank_jobs(request: RankJobsRequest) -> RankJobsResponse:
    if not request.jobs:
        return RankJobsResponse(result=rank_jobs_heuristic(request))

    if is_configured():
        try:
            result = rank_jobs_ai(request)
            return RankJobsResponse(result=result)
        except AIProviderError:
            logger.warning("AI job ranking failed, falling back to heuristic", exc_info=True)

    return RankJobsResponse(result=rank_jobs_heuristic(request))
