"""PDF/DOCX resume rendering from a structured ResumeProfile (see
MASTER_REQUIREMENTS.md §36-§38). Deterministic, template-based rendering —
no AI involved once the profile content itself has been produced upstream.
"""

from __future__ import annotations

import io

from docx import Document
from fpdf import FPDF

from app.resume.models import ResumeProfile


def render_pdf(profile: ResumeProfile) -> bytes:
    pdf = FPDF()
    pdf.set_auto_page_break(auto=True, margin=15)
    pdf.add_page()

    pdf.set_font("Helvetica", "B", 16)
    pdf.cell(0, 10, profile.contact.name or "Resume", new_x="LMARGIN", new_y="NEXT")

    contact_line = " | ".join(
        v for v in [profile.contact.email, profile.contact.phone, profile.contact.location] if v
    )
    if contact_line:
        pdf.set_font("Helvetica", "", 10)
        pdf.cell(0, 6, contact_line, new_x="LMARGIN", new_y="NEXT")
    pdf.ln(4)

    if profile.summary:
        _pdf_heading(pdf, "Summary")
        pdf.set_font("Helvetica", "", 10)
        pdf.multi_cell(0, 5, profile.summary, new_x="LMARGIN", new_y="NEXT")
        pdf.ln(2)

    if profile.skills:
        _pdf_heading(pdf, "Skills")
        pdf.set_font("Helvetica", "", 10)
        pdf.multi_cell(0, 5, ", ".join(profile.skills), new_x="LMARGIN", new_y="NEXT")
        pdf.ln(2)

    if profile.experiences:
        _pdf_heading(pdf, "Experience")
        for exp in profile.experiences:
            pdf.set_font("Helvetica", "B", 11)
            title_line = " - ".join(v for v in [exp.title, exp.company] if v) or "Experience"
            pdf.cell(0, 6, title_line, new_x="LMARGIN", new_y="NEXT")

            date_line = " - ".join(v for v in [exp.start_date, exp.end_date] if v)
            meta_line = " | ".join(v for v in [date_line, exp.location] if v)
            if meta_line:
                pdf.set_font("Helvetica", "I", 9)
                pdf.cell(0, 5, meta_line, new_x="LMARGIN", new_y="NEXT")

            pdf.set_font("Helvetica", "", 10)
            for bullet in exp.bullets:
                pdf.multi_cell(0, 5, f"- {bullet}", new_x="LMARGIN", new_y="NEXT")
            pdf.ln(2)

    if profile.education:
        _pdf_heading(pdf, "Education")
        pdf.set_font("Helvetica", "", 10)
        for entry in profile.education:
            pdf.multi_cell(0, 5, entry, new_x="LMARGIN", new_y="NEXT")
        pdf.ln(2)

    if profile.certifications:
        _pdf_heading(pdf, "Certifications")
        pdf.set_font("Helvetica", "", 10)
        for entry in profile.certifications:
            pdf.multi_cell(0, 5, entry, new_x="LMARGIN", new_y="NEXT")

    return bytes(pdf.output())


def _pdf_heading(pdf: FPDF, text: str) -> None:
    pdf.set_font("Helvetica", "B", 12)
    pdf.cell(0, 8, text, new_x="LMARGIN", new_y="NEXT")


def render_docx(profile: ResumeProfile) -> bytes:
    doc = Document()
    doc.add_heading(profile.contact.name or "Resume", level=1)

    contact_line = " | ".join(
        v for v in [profile.contact.email, profile.contact.phone, profile.contact.location] if v
    )
    if contact_line:
        doc.add_paragraph(contact_line)

    if profile.summary:
        doc.add_heading("Summary", level=2)
        doc.add_paragraph(profile.summary)

    if profile.skills:
        doc.add_heading("Skills", level=2)
        doc.add_paragraph(", ".join(profile.skills))

    if profile.experiences:
        doc.add_heading("Experience", level=2)
        for exp in profile.experiences:
            title_line = " - ".join(v for v in [exp.title, exp.company] if v) or "Experience"
            doc.add_heading(title_line, level=3)

            date_line = " - ".join(v for v in [exp.start_date, exp.end_date] if v)
            meta_line = " | ".join(v for v in [date_line, exp.location] if v)
            if meta_line:
                doc.add_paragraph(meta_line)

            for bullet in exp.bullets:
                doc.add_paragraph(bullet, style="List Bullet")

    if profile.education:
        doc.add_heading("Education", level=2)
        for entry in profile.education:
            doc.add_paragraph(entry)

    if profile.certifications:
        doc.add_heading("Certifications", level=2)
        for entry in profile.certifications:
            doc.add_paragraph(entry)

    buf = io.BytesIO()
    doc.save(buf)
    return buf.getvalue()
