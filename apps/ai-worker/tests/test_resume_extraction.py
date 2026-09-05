"""Tests for resume text extraction (PDF via PyMuPDF, DOCX via python-docx)."""

import io

import fitz
from docx import Document

from app.resume.extraction import UnsupportedResumeType, extract_text


def _build_pdf_bytes(text: str) -> bytes:
    doc = fitz.open()
    page = doc.new_page()
    page.insert_text((72, 72), text)
    return doc.tobytes()


def _build_docx_bytes(paragraphs: list[str]) -> bytes:
    document = Document()
    for paragraph in paragraphs:
        document.add_paragraph(paragraph)
    buffer = io.BytesIO()
    document.save(buffer)
    return buffer.getvalue()


def test_extract_pdf_text() -> None:
    pdf_bytes = _build_pdf_bytes("Jordan Rivera - Backend Engineer")
    text = extract_text(pdf_bytes, "application/pdf")
    assert "Jordan Rivera" in text


def test_extract_docx_text() -> None:
    docx_bytes = _build_docx_bytes(["Jordan Rivera", "Backend Engineer"])
    text = extract_text(
        docx_bytes,
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    )
    assert "Jordan Rivera" in text
    assert "Backend Engineer" in text


def test_extract_unsupported_mime_type_raises() -> None:
    try:
        extract_text(b"not a resume", "text/plain")
        raise AssertionError("expected UnsupportedResumeType")
    except UnsupportedResumeType:
        pass
