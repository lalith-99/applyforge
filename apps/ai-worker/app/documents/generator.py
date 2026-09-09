"""Deterministic ATS-friendly PDF/DOCX rendering for ResumeProfile.

Document rendering intentionally does not call an LLM. The template is
one-page-first: compact margins, categorized skills, dense experience rows,
and adaptive PDF typography before allowing a second page.
"""

from __future__ import annotations

import io
import re
import unicodedata
from dataclasses import dataclass

from docx import Document
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_LINE_SPACING, WD_TAB_ALIGNMENT
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Inches, Pt
from fpdf import FPDF

from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile

_CORE_FONTS_ENCODING = "cp1252"
_PDF_FONT = "Times"
_TEXT_COLOR = (25, 25, 25)
_MUTED_COLOR = (85, 85, 85)
_ACCENT_COLOR = (24, 55, 84)
_RULE_COLOR = (90, 90, 90)
_LINK_COLOR = (5, 76, 143)


@dataclass(frozen=True)
class _PDFStyle:
    left_margin: float
    top_margin: float
    right_margin: float
    bottom_margin: float
    name_size: float
    headline_size: float
    contact_size: float
    section_size: float
    body_size: float
    skill_size: float
    role_size: float
    meta_size: float
    line_height: float
    bullet_indent: float
    section_gap: float
    role_gap: float


_SPACIOUS = _PDFStyle(
    left_margin=9.0,
    top_margin=6.0,
    right_margin=9.0,
    bottom_margin=6.5,
    name_size=16.2,
    headline_size=9.9,
    contact_size=8.8,
    section_size=10.4,
    body_size=9.65,
    skill_size=8.95,
    role_size=9.7,
    meta_size=8.6,
    line_height=4.55,
    bullet_indent=3.9,
    section_gap=1.1,
    role_gap=0.75,
)


_RELAXED = _PDFStyle(
    left_margin=9.0,
    top_margin=6.0,
    right_margin=9.0,
    bottom_margin=6.5,
    name_size=16.0,
    headline_size=9.7,
    contact_size=8.7,
    section_size=10.2,
    body_size=9.35,
    skill_size=8.7,
    role_size=9.4,
    meta_size=8.4,
    line_height=4.3,
    bullet_indent=3.8,
    section_gap=0.9,
    role_gap=0.6,
)


_COMPACT = _PDFStyle(
    left_margin=9.0,
    top_margin=6.0,
    right_margin=9.0,
    bottom_margin=6.5,
    name_size=15.5,
    headline_size=9.5,
    contact_size=8.5,
    section_size=10.0,
    body_size=9.0,
    skill_size=8.5,
    role_size=9.2,
    meta_size=8.2,
    line_height=4.05,
    bullet_indent=3.8,
    section_gap=0.65,
    role_gap=0.45,
)

_TIGHT = _PDFStyle(
    left_margin=8.0,
    top_margin=5.5,
    right_margin=8.0,
    bottom_margin=6.0,
    name_size=14.5,
    headline_size=8.8,
    contact_size=8.0,
    section_size=9.4,
    body_size=8.2,
    skill_size=7.9,
    role_size=8.5,
    meta_size=7.7,
    line_height=3.65,
    bullet_indent=3.4,
    section_gap=0.45,
    role_gap=0.3,
)


def _sanitize_for_pdf(text: str) -> str:
    normalized = unicodedata.normalize("NFKD", text)
    return normalized.encode(_CORE_FONTS_ENCODING, errors="ignore").decode(_CORE_FONTS_ENCODING)


def _sanitize_optional(text: str | None) -> str | None:
    return _sanitize_for_pdf(text) if text else text


def _sanitize_profile_for_pdf(profile: ResumeProfile) -> ResumeProfile:
    return ResumeProfile(
        contact=ContactInfo(
            name=_sanitize_optional(profile.contact.name),
            headline=_sanitize_optional(profile.contact.headline),
            email=_sanitize_optional(profile.contact.email),
            phone=_sanitize_optional(profile.contact.phone),
            location=_sanitize_optional(profile.contact.location),
            linkedin_url=profile.contact.linkedin_url,
        ),
        summary=_sanitize_optional(profile.summary),
        skills=[_sanitize_for_pdf(s) for s in profile.skills],
        experiences=[
            ExperienceEntry(
                company=_sanitize_optional(exp.company),
                title=_sanitize_optional(exp.title),
                start_date=_sanitize_optional(exp.start_date),
                end_date=_sanitize_optional(exp.end_date),
                location=_sanitize_optional(exp.location),
                bullets=[_sanitize_for_pdf(b) for b in exp.bullets],
                detected_skills=exp.detected_skills,
                technologies=exp.technologies,
            )
            for exp in profile.experiences
        ],
        education=[_sanitize_for_pdf(e) for e in profile.education],
        certifications=[_sanitize_for_pdf(c) for c in profile.certifications],
    )


def render_pdf(profile: ResumeProfile) -> bytes:
    """Render one page with readable typography and balanced vertical fill."""
    sanitized = _sanitize_profile_for_pdf(profile)
    pdf = _render_pdf_document(sanitized, _COMPACT)

    if len(pdf.pages) > 1:
        pdf = _render_pdf_document(sanitized, _TIGHT)
        return bytes(pdf.output())

    # If a compact one-page resume leaves a conspicuous blank band at the
    # bottom, use a slightly more readable/roomy style. Keep it only when it
    # still fits on one page; we never stretch content onto a second page just
    # to consume whitespace.
    remaining = pdf.h - pdf.b_margin - pdf.get_y()
    if remaining > 30.0:
        spacious = _render_pdf_document(sanitized, _SPACIOUS)
        if len(spacious.pages) == 1:
            pdf = spacious
            return bytes(pdf.output())

    if remaining > 22.0:
        relaxed = _render_pdf_document(sanitized, _RELAXED)
        if len(relaxed.pages) == 1:
            pdf = relaxed

    return bytes(pdf.output())


def _render_pdf_document(profile: ResumeProfile, style: _PDFStyle) -> FPDF:
    pdf = FPDF()
    pdf.core_fonts_encoding = _CORE_FONTS_ENCODING
    pdf.set_margins(style.left_margin, style.top_margin, style.right_margin)
    pdf.set_auto_page_break(auto=True, margin=style.bottom_margin)
    pdf.add_page()

    _render_header(pdf, profile, style)

    if profile.summary:
        _section_heading(pdf, "Professional Summary", style)
        pdf.set_font(_PDF_FONT, "", style.body_size)
        pdf.set_text_color(*_TEXT_COLOR)
        pdf.multi_cell(
            0,
            style.line_height,
            profile.summary,
            align="L",
            new_x="LMARGIN",
            new_y="NEXT",
        )
        pdf.ln(style.section_gap)

    if profile.skills:
        _section_heading(pdf, "Technical Skills", style)
        _render_skill_groups(pdf, profile.skills, style)
        pdf.ln(style.section_gap)

    if profile.experiences:
        _section_heading(pdf, "Professional Experience", style)
        for index, exp in enumerate(profile.experiences):
            if index:
                pdf.ln(style.role_gap)
            _experience_heading(pdf, exp, style)
            pdf.set_font(_PDF_FONT, "", style.body_size)
            pdf.set_text_color(*_TEXT_COLOR)
            for bullet in exp.bullets:
                _bullet_item(pdf, bullet, style)
        pdf.ln(style.section_gap)

    if profile.education:
        _section_heading(pdf, "Education", style)
        pdf.set_font(_PDF_FONT, "", style.body_size)
        pdf.set_text_color(*_TEXT_COLOR)
        for entry in _compact_education_entries(profile.education):
            pdf.multi_cell(
                0,
                style.line_height,
                entry,
                align="L",
                new_x="LMARGIN",
                new_y="NEXT",
            )
        pdf.ln(style.section_gap)

    if profile.certifications:
        _section_heading(pdf, "Certifications", style)
        pdf.set_font(_PDF_FONT, "", style.body_size)
        pdf.set_text_color(*_TEXT_COLOR)
        for entry in profile.certifications:
            pdf.multi_cell(
                0,
                style.line_height,
                entry,
                align="L",
                new_x="LMARGIN",
                new_y="NEXT",
            )

    return pdf


def _render_header(pdf: FPDF, profile: ResumeProfile, style: _PDFStyle) -> None:
    content_width = pdf.w - pdf.l_margin - pdf.r_margin

    pdf.set_font(_PDF_FONT, "B", style.name_size)
    pdf.set_text_color(*_TEXT_COLOR)
    pdf.cell(
        content_width,
        6.0,
        profile.contact.name or "Resume",
        align="C",
        new_x="LMARGIN",
        new_y="NEXT",
    )

    if profile.contact.headline:
        pdf.set_font(_PDF_FONT, "B", style.headline_size)
        pdf.set_text_color(*_TEXT_COLOR)
        pdf.multi_cell(
            content_width,
            4.0,
            profile.contact.headline,
            align="C",
            new_x="LMARGIN",
            new_y="NEXT",
        )

    _render_contact_line(pdf, profile.contact, style)
    pdf.ln(0.6)


def _render_contact_line(pdf: FPDF, contact: ContactInfo, style: _PDFStyle) -> None:
    segments: list[tuple[str, str | None]] = []
    for value in [contact.location, contact.phone, contact.email]:
        if value:
            segments.append((value, None))
    if contact.linkedin_url:
        segments.append(("LinkedIn", contact.linkedin_url))

    if not segments:
        return

    pdf.set_font(_PDF_FONT, "", style.contact_size)
    separator = " | "
    widths: list[float] = []
    for index, (label, _) in enumerate(segments):
        widths.append(pdf.get_string_width(label))
        if index < len(segments) - 1:
            widths.append(pdf.get_string_width(separator))
    total_width = sum(widths)
    start_x = max(pdf.l_margin, (pdf.w - total_width) / 2)
    pdf.set_x(start_x)

    for index, (label, link) in enumerate(segments):
        if link:
            pdf.set_font(_PDF_FONT, "U", style.contact_size)
            pdf.set_text_color(*_LINK_COLOR)
            pdf.cell(pdf.get_string_width(label), 3.8, label, link=link, new_x="RIGHT")
            pdf.set_font(_PDF_FONT, "", style.contact_size)
            pdf.set_text_color(*_MUTED_COLOR)
        else:
            pdf.set_text_color(*_MUTED_COLOR)
            pdf.cell(pdf.get_string_width(label), 3.8, label, new_x="RIGHT")

        if index < len(segments) - 1:
            pdf.cell(pdf.get_string_width(separator), 3.8, separator, new_x="RIGHT")

    pdf.ln(3.8)
    pdf.set_x(pdf.l_margin)


def _section_heading(pdf: FPDF, text: str, style: _PDFStyle) -> None:
    pdf.set_font(_PDF_FONT, "B", style.section_size)
    pdf.set_text_color(*_TEXT_COLOR)
    pdf.cell(0, 4.2, text.upper(), new_x="LMARGIN", new_y="NEXT")
    pdf.set_draw_color(*_RULE_COLOR)
    pdf.set_line_width(0.2)
    pdf.line(pdf.l_margin, pdf.get_y(), pdf.w - pdf.r_margin, pdf.get_y())
    pdf.ln(0.55)


_SKILL_CATEGORY_ORDER = (
    "Languages",
    "Backend",
    "Databases & Messaging",
    "Cloud & DevOps",
    "Frontend",
    "Security",
    "Testing & Tools",
    "AI / GenAI",
    "Other",
)


def _skill_category(skill: str) -> str:
    value = skill.lower().strip()

    if any(
        token in value
        for token in (
            "oracle",
            "postgres",
            "mysql",
            "mongodb",
            "redis",
            "kafka",
            "database",
            "index optimization",
            "sql optimization",
            "asynchronous processing",
        )
    ):
        return "Databases & Messaging"

    language_names = (
        "java",
        "python",
        "javascript",
        "typescript",
        "sql",
        "bash",
        "kotlin",
        "go",
        "golang",
        "c",
        "c++",
        "c#",
    )
    if value in language_names or any(
        re.fullmatch(rf"{re.escape(language)}(?:\s+\d+(?:\.\d+)*)?", value)
        for language in language_names
    ):
        return "Languages"

    if any(
        token in value
        for token in (
            "spring",
            "j2ee",
            "rest",
            "microservice",
            "jpa",
            "hibernate",
            "service layer",
            "design pattern",
            "servlet",
            "jsp",
        )
    ):
        return "Backend"
    if any(
        token in value
        for token in (
            "aws",
            "amazon ecs",
            "amazon s3",
            "route 53",
            "lambda",
            "kubernetes",
            "docker",
            "terraform",
            "jenkins",
            "maven",
            "ci/cd",
            "azure",
            "gcp",
            "gce",
            "openshift",
            "linux",
            "ansible",
            "chef",
            "puppet",
            "travis",
            "gitlab ci",
            "github actions",
            "helm",
            "argo cd",
            "argocd",
        )
    ):
        return "Cloud & DevOps"
    if any(
        token in value
        for token in (
            "angular",
            "react",
            "html",
            "css",
            "responsive ui",
            "form validation",
            "section 508",
            "frontend",
        )
    ):
        return "Frontend"
    if any(token in value for token in ("oauth", "saml", "rbac", "pci", "security")):
        return "Security"
    if any(
        token in value
        for token in (
            "junit",
            "selenium",
            "jest",
            "tdd",
            "git",
            "jira",
            "agile",
            "scrum",
            "safe",
            "code review",
        )
    ):
        return "Testing & Tools"
    if any(
        token in value
        for token in (
            "rag",
            "langchain",
            "bedrock",
            "openai",
            "llm",
            "semantic retrieval",
            "claude",
            "genai",
        )
    ):
        return "AI / GenAI"
    return "Other"


def _group_skills(skills: list[str]) -> list[tuple[str, list[str]]]:
    grouped: dict[str, list[str]] = {name: [] for name in _SKILL_CATEGORY_ORDER}
    for skill in skills:
        grouped[_skill_category(skill)].append(skill)
    return [(name, grouped[name]) for name in _SKILL_CATEGORY_ORDER if grouped[name]]


def _render_skill_groups(pdf: FPDF, skills: list[str], style: _PDFStyle) -> None:
    groups = _group_skills(skills)
    content_width = pdf.w - pdf.l_margin - pdf.r_margin

    for label, values in groups:
        pdf.set_font(_PDF_FONT, "B", style.skill_size)
        pdf.set_text_color(*_TEXT_COLOR)
        # Use the width of each label instead of reserving the width of the
        # longest category for every row. This matches the source resume more
        # closely and gives shorter labels more horizontal room.
        label_width = pdf.get_string_width(f"{label}:") + 2.2
        pdf.cell(label_width, style.line_height, f"{label}:", new_x="RIGHT", new_y="TOP")
        pdf.set_font(_PDF_FONT, "", style.skill_size)
        # Keep each multi-word skill together so an orphan such as
        # "Asynchronous" / "Processing" cannot split across two visual lines.
        display_values = [value.replace(" ", "\xa0") for value in values]
        pdf.multi_cell(
            content_width - label_width,
            style.line_height,
            ", ".join(display_values),
            align="L",
            new_x="LMARGIN",
            new_y="NEXT",
        )


def _experience_heading(pdf: FPDF, exp: ExperienceEntry, style: _PDFStyle) -> None:
    left = " | ".join(v for v in [exp.company, exp.title] if v) or "Experience"
    dates = " - ".join(v for v in [exp.start_date, exp.end_date] if v)
    right = " | ".join(v for v in [dates, exp.location] if v)

    content_width = pdf.w - pdf.l_margin - pdf.r_margin
    pdf.set_font(_PDF_FONT, "I", style.meta_size)
    right_width = pdf.get_string_width(right) + 1.5 if right else 0
    left_width = max(35.0, content_width - right_width)

    role_size = style.role_size
    while role_size > 7.2:
        pdf.set_font(_PDF_FONT, "B", role_size)
        if pdf.get_string_width(left) <= left_width:
            break
        role_size -= 0.2

    pdf.set_font(_PDF_FONT, "B", role_size)
    pdf.set_text_color(*_TEXT_COLOR)
    if pdf.get_string_width(left) <= left_width:
        pdf.cell(left_width, style.line_height + 0.2, left, new_x="RIGHT", new_y="TOP")
        if right:
            pdf.set_font(_PDF_FONT, "B", style.meta_size)
            pdf.set_text_color(*_TEXT_COLOR)
            pdf.cell(
                right_width,
                style.line_height + 0.2,
                right,
                align="R",
                new_x="LMARGIN",
                new_y="NEXT",
            )
        else:
            pdf.ln(style.line_height + 0.2)
        return

    pdf.multi_cell(
        content_width,
        style.line_height,
        left,
        align="L",
        new_x="LMARGIN",
        new_y="NEXT",
    )
    if right:
        pdf.set_font(_PDF_FONT, "B", style.meta_size)
        pdf.set_text_color(*_TEXT_COLOR)
        pdf.cell(
            content_width,
            style.line_height,
            right,
            align="R",
            new_x="LMARGIN",
            new_y="NEXT",
        )


def _bullet_item(pdf: FPDF, text: str, style: _PDFStyle) -> None:
    x_start = pdf.l_margin
    pdf.set_x(x_start)
    pdf.cell(style.bullet_indent, style.line_height, "•", new_x="RIGHT", new_y="TOP")
    pdf.set_x(x_start + style.bullet_indent)
    available_width = pdf.w - pdf.r_margin - (x_start + style.bullet_indent)
    pdf.multi_cell(
        available_width,
        style.line_height,
        text,
        align="L",
        new_x="LMARGIN",
        new_y="NEXT",
    )


def _compact_education_entries(entries: list[str]) -> list[str]:
    if (
        len(entries) == 2
        and not any(char.isdigit() for char in entries[0])
        and any(char.isdigit() for char in entries[1])
    ):
        return [f"{entries[0]} | {entries[1]}"]
    return entries


def render_docx(profile: ResumeProfile) -> bytes:
    """Render a compact, single-column ATS-friendly DOCX."""
    doc = Document()
    _set_docx_base_style(doc)

    section = doc.sections[0]
    section.top_margin = Inches(0.35)
    section.bottom_margin = Inches(0.35)
    section.left_margin = Inches(0.45)
    section.right_margin = Inches(0.45)

    name = doc.add_paragraph()
    name.alignment = WD_ALIGN_PARAGRAPH.CENTER
    name.paragraph_format.space_after = Pt(0)
    run = name.add_run(profile.contact.name or "Resume")
    run.bold = True
    run.font.size = Pt(16)

    if profile.contact.headline:
        headline = doc.add_paragraph()
        headline.alignment = WD_ALIGN_PARAGRAPH.CENTER
        headline.paragraph_format.space_after = Pt(0)
        run = headline.add_run(profile.contact.headline)
        run.bold = True
        run.font.size = Pt(9)

    _add_docx_contact_line(doc, profile.contact)

    if profile.summary:
        _add_docx_section_heading(doc, "Professional Summary")
        _add_docx_body(doc, profile.summary)

    if profile.skills:
        _add_docx_section_heading(doc, "Technical Skills")
        for label, values in _group_skills(profile.skills):
            para = doc.add_paragraph()
            _compact_docx_paragraph(para)
            label_run = para.add_run(f"{label}: ")
            label_run.bold = True
            para.add_run(", ".join(values))

    if profile.experiences:
        _add_docx_section_heading(doc, "Professional Experience")
        for exp in profile.experiences:
            left = " | ".join(v for v in [exp.company, exp.title] if v) or "Experience"
            dates = " - ".join(v for v in [exp.start_date, exp.end_date] if v)
            right = " | ".join(v for v in [dates, exp.location] if v)
            _add_docx_title_with_trailing_date(doc, left, right)
            for bullet in exp.bullets:
                para = doc.add_paragraph(style="List Bullet")
                _compact_docx_paragraph(para)
                para.add_run(bullet)

    if profile.education:
        _add_docx_section_heading(doc, "Education")
        for entry in _compact_education_entries(profile.education):
            _add_docx_body(doc, entry)

    if profile.certifications:
        _add_docx_section_heading(doc, "Certifications")
        for entry in profile.certifications:
            _add_docx_body(doc, entry)

    buf = io.BytesIO()
    doc.save(buf)
    return buf.getvalue()


def _set_docx_base_style(doc: Document) -> None:
    normal = doc.styles["Normal"]
    normal.font.name = "Times New Roman"
    normal.font.size = Pt(9)
    normal.paragraph_format.space_after = Pt(0)
    normal.paragraph_format.line_spacing_rule = WD_LINE_SPACING.SINGLE

    list_bullet = doc.styles["List Bullet"]
    list_bullet.font.name = "Times New Roman"
    list_bullet.font.size = Pt(9)
    list_bullet.paragraph_format.space_after = Pt(0)


def _compact_docx_paragraph(para) -> None:
    para.paragraph_format.space_before = Pt(0)
    para.paragraph_format.space_after = Pt(0)
    para.paragraph_format.line_spacing_rule = WD_LINE_SPACING.SINGLE


def _add_docx_contact_line(doc: Document, contact: ContactInfo) -> None:
    para = doc.add_paragraph()
    para.alignment = WD_ALIGN_PARAGRAPH.CENTER
    _compact_docx_paragraph(para)

    values = [value for value in [contact.location, contact.phone, contact.email] if value]
    for index, value in enumerate(values):
        if index:
            para.add_run(" | ")
        para.add_run(value)

    if contact.linkedin_url:
        if values:
            para.add_run(" | ")
        _add_docx_hyperlink(para, "LinkedIn", contact.linkedin_url)


def _add_docx_hyperlink(paragraph, text: str, url: str) -> None:
    relationship_id = paragraph.part.relate_to(
        url,
        "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
        is_external=True,
    )
    hyperlink = OxmlElement("w:hyperlink")
    hyperlink.set(qn("r:id"), relationship_id)

    run = OxmlElement("w:r")
    props = OxmlElement("w:rPr")
    color = OxmlElement("w:color")
    color.set(qn("w:val"), "0563C1")
    underline = OxmlElement("w:u")
    underline.set(qn("w:val"), "single")
    props.append(color)
    props.append(underline)
    run.append(props)

    text_element = OxmlElement("w:t")
    text_element.text = text
    run.append(text_element)
    hyperlink.append(run)
    paragraph._p.append(hyperlink)


def _add_docx_section_heading(doc: Document, text: str) -> None:
    para = doc.add_paragraph()
    para.paragraph_format.space_before = Pt(3)
    para.paragraph_format.space_after = Pt(0)
    run = para.add_run(text.upper())
    run.bold = True
    run.font.size = Pt(10)


def _add_docx_body(doc: Document, text: str) -> None:
    para = doc.add_paragraph()
    _compact_docx_paragraph(para)
    para.add_run(text)


def _add_docx_title_with_trailing_date(doc: Document, title_text: str, date_text: str) -> None:
    para = doc.add_paragraph()
    _compact_docx_paragraph(para)
    section = doc.sections[0]
    text_width = section.page_width - section.left_margin - section.right_margin
    para.paragraph_format.tab_stops.add_tab_stop(text_width, WD_TAB_ALIGNMENT.RIGHT)

    title_run = para.add_run(title_text)
    title_run.bold = True

    if date_text:
        date_run = para.add_run(f"\t{date_text}")
        date_run.bold = True
