"""Job-description requirement extraction endpoint.

Called by the Go API when it needs JobRequirements for a job whose
content_hash hasn't been parsed yet (cached thereafter — see
docs/AI_PIPELINE.md cost-management notes).
"""

from __future__ import annotations

import logging

from fastapi import APIRouter

from app.jobs.models import (
    ClassifyRoleRequest,
    ClassifyRoleResponse,
    ParseRequirementsRequest,
    ParseRequirementsResponse,
)
from app.jobs.parsing import (
    classify_job_role,
    classify_job_role_ai,
    parse_job_requirements,
    parse_job_requirements_ai,
)
from app.providers.openai_provider import AIProviderError, is_configured

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/jobs", tags=["jobs"])


@router.post("/parse-requirements", response_model=ParseRequirementsResponse)
def parse_requirements(request: ParseRequirementsRequest) -> ParseRequirementsResponse:
    if is_configured():
        try:
            requirements = parse_job_requirements_ai(request.title, request.description)
            return ParseRequirementsResponse(requirements=requirements)
        except AIProviderError:
            logger.warning(
                "AI job requirements parsing failed, falling back to heuristic", exc_info=True
            )

    requirements = parse_job_requirements(request.title, request.description)
    return ParseRequirementsResponse(requirements=requirements)


@router.post("/classify-role", response_model=ClassifyRoleResponse)
def classify_role(request: ClassifyRoleRequest) -> ClassifyRoleResponse:
    if is_configured():
        try:
            result = classify_job_role_ai(request.title, request.description)
            return ClassifyRoleResponse(result=result)
        except AIProviderError:
            logger.warning("AI role classification failed, falling back to heuristic", exc_info=True)

    return ClassifyRoleResponse(result=classify_job_role(request.title, request.description))
