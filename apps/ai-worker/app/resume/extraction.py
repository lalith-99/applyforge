"""Selectable-text extraction from uploaded resume documents."""

from __future__ import annotations

import io

import fitz  # PyMuPDF
from docx import Document
from docx.opc.constants import RELATIONSHIP_TYPE as RT

SUPPORTED_MIME_TYPES = {
    "application/pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}


class UnsupportedResumeType(ValueError):
    pass


def extract_text(file_bytes: bytes, mime_type: str) -> str:
    """Extract raw selectable text plus useful external resume links.

    PDF/DOCX visible text often contains only an anchor such as "LinkedIn";
    the real target URL lives in document metadata/relationships. Preserve
    those targets in the extracted text so structured parsing can retain them.
    """
    if mime_type == "application/pdf":
        return _extract_pdf(file_bytes)
    if mime_type == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
        return _extract_docx(file_bytes)
    raise UnsupportedResumeType(f"unsupported mime type: {mime_type}")


def _extract_pdf(file_bytes: bytes) -> str:
    # Some PDFs embed fonts whose ToUnicode CMap maps ligature glyphs to the
    # wrong code point. Disabling ligature preservation lets PyMuPDF decompose
    # them via its glyph-name table instead.
    flags = fitz.TEXTFLAGS_TEXT & ~fitz.TEXT_PRESERVE_LIGATURES
    text_parts: list[str] = []
    links: list[str] = []
    with fitz.open(stream=file_bytes, filetype="pdf") as doc:
        for page in doc:
            text_parts.append(page.get_text(flags=flags))
            for link in page.get_links():
                uri = str(link.get("uri") or "").strip()
                if uri:
                    links.append(uri)

    return _append_external_links("\n".join(text_parts).strip(), links)


def _extract_docx(file_bytes: bytes) -> str:
    document = Document(io.BytesIO(file_bytes))
    text = "\n".join(p.text for p in document.paragraphs).strip()
    links = [
        rel.target_ref
        for rel in document.part.rels.values()
        if rel.reltype == RT.HYPERLINK and rel.is_external
    ]
    return _append_external_links(text, links)


def _append_external_links(text: str, links: list[str]) -> str:
    """Append normalized external links only when the URL isn't already text."""
    seen: set[str] = set()
    extra: list[str] = []
    lowered_text = text.lower()

    for raw in links:
        url = raw.strip()
        key = url.rstrip("/").lower()
        if not url or key in seen or key in lowered_text:
            continue
        seen.add(key)

        lower = url.lower()
        if "linkedin.com/" in lower:
            extra.append(f"LinkedIn URL: {url}")
        elif "github.com/" in lower:
            extra.append(f"GitHub URL: {url}")
        else:
            extra.append(f"External URL: {url}")

    if not extra:
        return text
    return (text.rstrip() + "\n" + "\n".join(extra)).strip()
