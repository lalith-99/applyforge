"""Job-description requirement extraction endpoint.

Called by the Go API when it needs JobRequirements for a job whose
content_hash hasn't been parsed yet (cached thereafter — see
docs/AI_PIPELINE.md cost-management notes).
"""

from __future__ import annotations

from fastapi import APIRouter

from app.jobs.models import ParseRequirementsRequest, ParseRequirementsResponse
from app.jobs.parsing import parse_job_requirements

router = APIRouter(prefix="/v1/jobs", tags=["jobs"])


@router.post("/parse-requirements", response_model=ParseRequirementsResponse)
def parse_requirements(request: ParseRequirementsRequest) -> ParseRequirementsResponse:
    requirements = parse_job_requirements(request.title, request.description)
    return ParseRequirementsResponse(requirements=requirements)
