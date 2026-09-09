"""Resume text-extraction and structured-parsing endpoints.

Called by the Go API's background worker during resume upload processing
(see apps/api/internal/resume). Not exposed to the browser directly.
"""

from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException, Response, UploadFile

from app.providers.openai_provider import (
    AIProviderError,
    apply_usage_headers,
    clear_usage_metadata,
    is_configured,
)
from app.resume.extraction import SUPPORTED_MIME_TYPES, UnsupportedResumeType, extract_text
from app.resume.models import ExtractResponse, ParseRequest, ParseResponse
from app.resume.faithful import parse_resume_text_faithful, reconcile_ai_profile\nfrom app.resume.parsing import parse_resume_text_ai

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/resumes", tags=["resumes"])


@router.post("/extract", response_model=ExtractResponse)
async def extract(file: UploadFile) -> ExtractResponse:
    if file.content_type not in SUPPORTED_MIME_TYPES:
        raise HTTPException(status_code=422, detail=f"unsupported mime type: {file.content_type}")

    file_bytes = await file.read()
    try:
        raw_text = extract_text(file_bytes, file.content_type)
    except UnsupportedResumeType as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc

    if not raw_text.strip():
        raise HTTPException(status_code=422, detail="no selectable text found in document")

    return ExtractResponse(raw_text=raw_text)


@router.post("/parse", response_model=ParseResponse)
def parse(request: ParseRequest, response: Response) -> ParseResponse:
    if is_configured():
        clear_usage_metadata()
        try:
            ai_profile = parse_resume_text_ai(request.raw_text)
            result = ParseResponse(
                profile=reconcile_ai_profile(request.raw_text, ai_profile)
            )
            apply_usage_headers(response)
            return result
        except AIProviderError:
            logger.warning("AI resume parsing failed, falling back to heuristic", exc_info=True)

    profile = parse_resume_text_faithful(request.raw_text)
    return ParseResponse(profile=profile)
