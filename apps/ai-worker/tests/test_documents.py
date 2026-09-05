"""Tests for PDF/DOCX resume document generation."""

from fastapi.testclient import TestClient

from app.documents.generator import render_docx, render_pdf
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
