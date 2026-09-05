"""Selectable-text extraction from uploaded resume documents."""

from __future__ import annotations

import io

import fitz  # PyMuPDF
from docx import Document

SUPPORTED_MIME_TYPES = {
    "application/pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}


class UnsupportedResumeType(ValueError):
    pass


def extract_text(file_bytes: bytes, mime_type: str) -> str:
    """Extract raw selectable text from a PDF or DOCX resume."""
    if mime_type == "application/pdf":
        return _extract_pdf(file_bytes)
    if mime_type == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
        return _extract_docx(file_bytes)
    raise UnsupportedResumeType(f"unsupported mime type: {mime_type}")


def _extract_pdf(file_bytes: bytes) -> str:
    # Some PDFs (certain resume templates/generators) embed fonts whose
    # ToUnicode CMap maps ligature glyphs (fi, ti, ffl, ...) to the wrong
    # code point. Preserving ligatures then trusts that broken mapping,
    # producing garbage like "Cer$fica$ons" instead of "Certifications".
    # Disabling ligature preservation makes PyMuPDF decompose ligatures via
    # its own glyph-name table instead.
    flags = fitz.TEXTFLAGS_TEXT & ~fitz.TEXT_PRESERVE_LIGATURES
    text_parts: list[str] = []
    with fitz.open(stream=file_bytes, filetype="pdf") as doc:
        for page in doc:
            text_parts.append(page.get_text(flags=flags))
    return "\n".join(text_parts).strip()


def _extract_docx(file_bytes: bytes) -> str:
    document = Document(io.BytesIO(file_bytes))
    return "\n".join(p.text for p in document.paragraphs).strip()
