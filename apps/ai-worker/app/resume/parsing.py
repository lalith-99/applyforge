"""Heuristic (non-LLM) structured resume parsing.

This is a deterministic, regex/keyword-based stand-in for the real AI-driven
parsing described in MASTER_REQUIREMENTS.md §11/§17. It exists so the full
upload -> extract -> parse -> review pipeline is functional end-to-end without
an AI_API_KEY configured. Swap in a real AIProvider.ParseResume behind this
same function signature once one is available — see app/providers/.
"""

from __future__ import annotations

import re

from app.core.skills_dictionary import canonical_skills
from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile

_EMAIL_RE = re.compile(r"[\w.+-]+@[\w-]+\.[\w.-]+")
_PHONE_RE = re.compile(r"(\+?\d[\d\-. ()]{8,}\d)")
_DATE_RANGE_RE = re.compile(
    r"((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+)?(19|20)\d{2}\s*[-\u2013\u2014]\s*"
    r"((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+)?((19|20)\d{2}|[Pp]resent|[Cc]urrent)",
    re.IGNORECASE,
)

_SECTION_HEADERS = {
    "summary": "summary",
    "profile": "summary",
    "objective": "summary",
    "about": "summary",
    "skills": "skills",
    "technical skills": "skills",
    "core competencies": "skills",
    "experience": "experience",
    "work experience": "experience",
    "professional experience": "experience",
    "employment": "experience",
    "employment history": "experience",
    "education": "education",
    "certifications": "certifications",
    "certificates": "certifications",
}

_SKILL_TOKEN_SPLIT_RE = re.compile(r"[,|/\u2022\u2023\u25E6\u2043\u2219•·]+")


def _detect_header(line: str) -> str | None:
    normalized = line.strip().strip(":").lower()
    if not normalized or len(normalized) > 40:
        return None
    return _SECTION_HEADERS.get(normalized)


def _split_sections(lines: list[str]) -> dict[str, list[str]]:
    sections: dict[str, list[str]] = {"preamble": []}
    current = "preamble"
    for line in lines:
        header = _detect_header(line)
        if header:
            current = header
            sections.setdefault(current, [])
            continue
        sections.setdefault(current, []).append(line)
    return sections


def _extract_skills_from_text(text: str) -> list[str]:
    found: list[str] = []
    lowered = text.lower()
    for skill in canonical_skills():
        pattern = r"(?<![\w+#.-])" + re.escape(skill.lower()) + r"(?![\w+#-])"
        if re.search(pattern, lowered):
            found.append(skill)
    return found


def _parse_skills_section(lines: list[str]) -> list[str]:
    text = " ".join(lines)
    tokens = [t.strip() for t in _SKILL_TOKEN_SPLIT_RE.split(text) if t.strip()]
    skills: list[str] = []
    seen: set[str] = set()

    for token in tokens:
        canonical = None
        for skill in canonical_skills():
            if skill.lower() == token.lower():
                canonical = skill
                break
        if canonical and canonical.lower() not in seen:
            skills.append(canonical)
            seen.add(canonical.lower())

    # Also sweep the raw text in case skills are embedded in prose rather than a list.
    for skill in _extract_skills_from_text(text):
        if skill.lower() not in seen:
            skills.append(skill)
            seen.add(skill.lower())

    return skills


def _parse_experience_section(lines: list[str]) -> list[ExperienceEntry]:
    non_empty = [line for line in lines if line.strip()]
    if not non_empty:
        return []

    # Group lines into blocks: a new block starts at a line containing a date
    # range, or a bullet-free "header-looking" line following a bullet line.
    blocks: list[list[str]] = []
    current_block: list[str] = []
    for line in non_empty:
        is_bullet = line.strip().startswith(("-", "*", "\u2022", "\u2023"))
        starts_new = bool(_DATE_RANGE_RE.search(line)) and current_block and not is_bullet
        if starts_new:
            blocks.append(current_block)
            current_block = [line]
        else:
            current_block.append(line)
    if current_block:
        blocks.append(current_block)

    entries: list[ExperienceEntry] = []
    for block in blocks:
        header_line = block[0]
        bullets = [
            line.strip().lstrip("-*\u2022\u2023").strip()
            for line in block[1:]
            if line.strip().startswith(("-", "*", "\u2022", "\u2023"))
        ]

        date_match = _DATE_RANGE_RE.search(header_line)
        start_date = end_date = None
        header_text = header_line
        if date_match:
            start_date, end_date = _split_dates(date_match.group(0))
            header_text = header_line[: date_match.start()].strip(" -\u2013\u2014|,()")

        title, company = _split_title_company(header_text)
        bullet_text = " ".join(bullets)
        entries.append(
            ExperienceEntry(
                company=company,
                title=title,
                start_date=start_date,
                end_date=end_date,
                bullets=bullets,
                detected_skills=_extract_skills_from_text(bullet_text),
                technologies=_extract_skills_from_text(bullet_text),
            )
        )
    return entries


def _split_dates(date_range: str) -> tuple[str | None, str | None]:
    parts = re.split(r"[-\u2013\u2014]", date_range, maxsplit=1)
    if len(parts) == 2:
        return parts[0].strip(), parts[1].strip()
    return date_range.strip(), None


def _split_title_company(header: str) -> tuple[str | None, str | None]:
    for sep in (" at ", " @ ", " - ", " \u2013 ", "|", ","):
        if sep in header:
            left, right = header.split(sep, 1)
            return left.strip() or None, right.strip() or None
    return header.strip() or None, None


def parse_resume_text(raw_text: str) -> ResumeProfile:
    lines = raw_text.splitlines()

    email_match = _EMAIL_RE.search(raw_text)
    phone_match = _PHONE_RE.search(raw_text)
    name = next((line.strip() for line in lines[:5] if line.strip() and "@" not in line), None)

    sections = _split_sections(lines)

    summary_lines = sections.get("summary") or sections.get("preamble", [])
    summary = " ".join(line.strip() for line in summary_lines if line.strip()).strip() or None

    skills = _parse_skills_section(sections.get("skills", []))
    experiences = _parse_experience_section(sections.get("experience", []))
    education = [line.strip() for line in sections.get("education", []) if line.strip()]
    certifications = [line.strip() for line in sections.get("certifications", []) if line.strip()]

    return ResumeProfile(
        contact=ContactInfo(
            name=name,
            email=email_match.group(0) if email_match else None,
            phone=phone_match.group(0) if phone_match else None,
        ),
        summary=summary,
        skills=skills,
        experiences=experiences,
        education=education,
        certifications=certifications,
    )


def parse_resume_text_ai(raw_text: str) -> ResumeProfile:
    """Real LLM-backed resume parsing. Raises AIProviderError (see
    app/providers/openai_provider.py) on any failure — callers must fall
    back to parse_resume_text() above."""
    from app.providers.openai_provider import structured_completion

    system = (
        "You are an expert resume parser. Extract structured information from the resume text into "
        "the exact schema provided. Be strictly faithful to the source text: do not invent employers, "
        "titles, dates, skills, or bullet content that isn't present. Preserve bullet wording closely, "
        "only cleaning up obvious PDF-extraction artifacts (broken ligatures, stray whitespace). For each "
        "experience, detected_skills should list only skills clearly evidenced by that experience's own "
        "bullets, and technologies should list specific tools/technologies named in those bullets."
    )
    user = f"Resume text:\n\n{raw_text}"
    return structured_completion(system, user, ResumeProfile)
