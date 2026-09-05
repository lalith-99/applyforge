"""Tests for PDF/DOCX resume document generation."""

from fastapi.testclient import TestClient

from app.documents.generator import _sanitize_for_pdf, render_docx, render_pdf
from app.main import app
from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile

client = TestClient(app)


def _sample_profile() -> ResumeProfile:
    return ResumeProfile(
        contact=ContactInfo(
            name="Ada Lovelace", email="ada@example.com", phone="555-0100", location="NYC"
        ),
        summary="Backend engineer with 6 years of experience.",
        skills=["Go", "PostgreSQL", "Kafka"],
        experiences=[
            ExperienceEntry(
                company="Acme",
                title="Senior Engineer",
                start_date="2020",
                end_date="Present",
                location="Remote",
                bullets=["Built event pipelines using Kafka", "Led a team of 4 engineers"],
                detected_skills=["kafka"],
                technologies=["Go", "Kafka"],
            )
        ],
        education=["B.S. Computer Science"],
        certifications=["AWS Certified Developer"],
    )


def test_render_pdf_produces_valid_pdf_bytes() -> None:
    data = render_pdf(_sample_profile())
    assert data.startswith(b"%PDF")
    assert len(data) > 100


def test_render_docx_produces_valid_zip_bytes() -> None:
    data = render_docx(_sample_profile())
    # DOCX files are zip archives, which start with the "PK" local file header.
    assert data[:2] == b"PK"
    assert len(data) > 100


def test_render_pdf_handles_minimal_profile() -> None:
    minimal = ResumeProfile()
    data = render_pdf(minimal)
    assert data.startswith(b"%PDF")


def test_render_pdf_handles_ligatures_and_smart_punctuation() -> None:
    # Real-world PDF text extraction often yields ligature glyphs (e.g. the
    # single-character "fi" in "Certifications") and smart quotes/dashes that
    # fpdf2's built-in latin-1-only core fonts can't render directly.
    profile = ResumeProfile(
        summary="Certi\ufb01cations and Technical Skills: \u201cquoted\u201d \u2013 en \u2014 em",
    )
    data = render_pdf(profile)
    assert data.startswith(b"%PDF")


def test_sanitize_for_pdf_decomposes_fi_ligature() -> None:
    assert _sanitize_for_pdf("Certi\ufb01cations") == "Certifications"


def test_sanitize_for_pdf_preserves_smart_punctuation_via_cp1252() -> None:
    # cp1252 (a superset of latin-1) natively supports these, so they render
    # as-is rather than being degraded to plain ASCII.
    text = "\u201cquoted\u201d \u2013 dash \u2014 em"
    assert _sanitize_for_pdf(text) == text


def test_documents_pdf_endpoint() -> None:
    response = client.post("/v1/documents/pdf", json=_sample_profile().model_dump())
    assert response.status_code == 200
    assert response.headers["content-type"] == "application/pdf"
    assert response.content.startswith(b"%PDF")


def test_documents_docx_endpoint() -> None:
    response = client.post("/v1/documents/docx", json=_sample_profile().model_dump())
    assert response.status_code == 200
    assert "wordprocessingml" in response.headers["content-type"]
    assert response.content[:2] == b"PK"
