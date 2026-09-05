"""FastAPI routes for PDF/DOCX resume document generation (Phase 9)."""

from __future__ import annotations

from fastapi import APIRouter, Response

from app.documents.generator import render_docx, render_pdf
from app.resume.models import ResumeProfile

router = APIRouter(prefix="/v1/documents", tags=["documents"])


@router.post("/pdf")
def generate_pdf(profile: ResumeProfile) -> Response:
    return Response(content=render_pdf(profile), media_type="application/pdf")


@router.post("/docx")
def generate_docx(profile: ResumeProfile) -> Response:
    return Response(
        content=render_docx(profile),
        media_type="application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    )
