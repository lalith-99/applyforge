"""Source-faithful resume parsing and AI-output repair.

This module is intentionally conservative: when a resume has explicit standard
sections, preserve the source wording/versions/bullets exactly instead of
normalizing them into a smaller skill vocabulary or dropping wrapped lines.
The LLM parser remains useful for unusual layouts, but its output is reconciled
against deterministic source evidence before persistence.
"""

from __future__ import annotations

import re

from app.core.skills_dictionary import canonical_skills
from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile

_EMAIL_RE = re.compile(r"[\w.+-]+@[\w-]+\.[\w.-]+")
_PHONE_RE = re.compile(r"(\(\d{3}\)[\d\-. ]{6,}\d|\+?\d[\d\-. ()]{8,}\d)")
_LINKEDIN_RE = re.compile(
    r"https?://(?:www\.)?linkedin\.com/(?:in|pub)/[^\s|)>\]]+",
    re.IGNORECASE,
)
_DATE_RANGE_RE = re.compile(
    r"((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+)?"
    r"(19|20)\d{2}\s*[-\u2013\u2014]\s*"
    r"((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+)?"
    r"((19|20)\d{2}|[Pp]resent|[Cc]urrent)",
    re.IGNORECASE,
)

_SECTION_HEADERS = {
    "summary": "summary",
    "professional summary": "summary",
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
_SECTION_PATTERN = re.compile(
    r"\b("
    + "|".join(
        sorted((re.escape(key.upper()) for key in _SECTION_HEADERS), key=len, reverse=True)
    )
    + r")\b"
)
_TITLE_HINTS = (
    "engineer",
    "developer",
    "architect",
    "consultant",
    "analyst",
    "manager",
    "lead",
    "specialist",
    "administrator",
    "scientist",
    "programmer",
    "director",
)


def _explode_inline_section_headers(raw_text: str) -> list[str]:
    """Split uppercase section headers even when PDF extraction glues them to text."""
    lines: list[str] = []
    for source_line in raw_text.splitlines():
        cursor = 0
        matched = False
        for match in _SECTION_PATTERN.finditer(source_line):
            token = match.group(1)
            # Only split exact uppercase source text. This avoids breaking normal
            # prose such as "professional experience" inside a summary sentence.
            if source_line[match.start() : match.end()] != token:
                continue
            before = source_line[cursor : match.start()].strip()
            if before:
                lines.append(before)
            lines.append(token)
            cursor = match.end()
            matched = True
        tail = source_line[cursor:].strip()
        if tail:
            lines.append(tail)
        elif not matched and not source_line.strip():
            lines.append("")
    return lines


def _detect_header(line: str) -> str | None:
    normalized = line.strip().strip(":").lower()
    if not normalized or len(normalized) > 50:
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


def _join_wrapped_lines(lines: list[str]) -> str | None:
    result = ""
    for raw in lines:
        line = " ".join(raw.split()).strip()
        if not line:
            continue
        if result.endswith("-") and line[0].isalnum():
            result += line
        elif result:
            result += " " + line
        else:
            result = line
    return result or None


def _extract_headline(lines: list[str], name: str | None) -> str | None:
    if not name:
        return None
    try:
        name_index = next(i for i, line in enumerate(lines[:8]) if line.strip() == name)
    except StopIteration:
        return None

    for line in lines[name_index + 1 : name_index + 4]:
        value = " ".join(line.split()).strip()
        if not value or "@" in value or _PHONE_RE.search(value) or _detect_header(value):
            continue
        if "|" in value and not re.search(
            r"\b(?:USA|United States|Remote|India)\b", value, re.IGNORECASE
        ):
            return value
    return None


def _extract_location(lines: list[str]) -> str | None:
    for line in lines[:8]:
        if "@" not in line and not _PHONE_RE.search(line):
            continue
        for segment in (part.strip() for part in line.split("|")):
            if (
                not segment
                or "@" in segment
                or _PHONE_RE.search(segment)
                or "linkedin" in segment.lower()
                or "github" in segment.lower()
            ):
                continue
            if "," in segment or re.search(
                r"\b(?:USA|United States|Remote|India)\b", segment, re.IGNORECASE
            ):
                return segment
    return None


def _parse_skills_section(lines: list[str]) -> list[str]:
    """Preserve exact source skill labels, versions, and product names."""
    category_re = re.compile(r"^([A-Za-z][A-Za-z0-9 /&+.-]{1,40}):\s*(.*)$")
    groups: list[str] = []
    current: list[str] | None = None

    for raw in lines:
        line = " ".join(raw.split()).strip()
        if not line:
            continue
        match = category_re.match(line)
        if match:
            if current:
                groups.append(" ".join(current))
            current = [match.group(2).strip()]
        elif current is not None:
            current.append(line)
        else:
            groups.append(line)

    if current:
        groups.append(" ".join(current))

    skills: list[str] = []
    seen: set[str] = set()
    for group in groups:
        for token in re.split(r"[,;|\u2022\u00b7]+", group):
            value = " ".join(token.split()).strip(" -")
            if not value:
                continue
            key = value.lower()
            if key not in seen:
                seen.add(key)
                skills.append(value)
    return skills


def _extract_skills_from_text(text: str) -> list[str]:
    found: list[str] = []
    lowered = text.lower()
    for skill in canonical_skills():
        pattern = r"(?<![\w+#.-])" + re.escape(skill.lower()) + r"(?![\w+#-])"
        if re.search(pattern, lowered):
            found.append(skill)
    return found


def _split_dates(date_range: str) -> tuple[str | None, str | None]:
    parts = re.split(r"[-\u2013\u2014]", date_range, maxsplit=1)
    if len(parts) == 2:
        return parts[0].strip(), parts[1].strip()
    return date_range.strip(), None


def _looks_like_title(value: str) -> bool:
    lowered = value.lower()
    return any(token in lowered for token in _TITLE_HINTS)


def _parse_experience_header(
    line: str,
) -> tuple[str | None, str | None, str | None, str | None, str | None]:
    date_match = _DATE_RANGE_RE.search(line)
    before = line
    after = ""
    start_date = end_date = None
    if date_match:
        start_date, end_date = _split_dates(date_match.group(0))
        before = line[: date_match.start()].strip(" -\u2013\u2014|,()")
        after = line[date_match.end() :].strip()

    location = after.strip(" /|,-\u2013\u2014()") or None
    parts = [part.strip() for part in before.split("|") if part.strip()]

    company = title = None
    if len(parts) >= 2:
        left, right = parts[0], parts[1]
        if _looks_like_title(left) and not _looks_like_title(right):
            title, company = left, right
        elif _looks_like_title(right):
            company, title = left, right
        else:
            company, title = left, right
    elif len(parts) == 1:
        if _looks_like_title(parts[0]):
            title = parts[0]
        else:
            company = parts[0]

    return company, title, start_date, end_date, location


def _append_continuation(current: str, continuation: str) -> str:
    continuation = " ".join(continuation.split()).strip()
    if not current:
        return continuation
    if current.endswith("-") and continuation and continuation[0].isalnum():
        return current + continuation
    return current.rstrip() + " " + continuation


def _parse_experience_section(lines: list[str]) -> list[ExperienceEntry]:
    blocks: list[tuple[str, list[str]]] = []
    header: str | None = None
    body: list[str] = []

    for raw in lines:
        line = raw.strip()
        if not line:
            continue
        is_bullet = line.startswith(("-", "*", "\u2022", "\u2023"))
        is_header = bool(_DATE_RANGE_RE.search(line)) and not is_bullet
        if is_header:
            if header is not None:
                blocks.append((header, body))
            header = line
            body = []
        elif header is not None:
            body.append(line)

    if header is not None:
        blocks.append((header, body))

    entries: list[ExperienceEntry] = []
    for header_line, body_lines in blocks:
        bullets: list[str] = []
        current_bullet: str | None = None
        for line in body_lines:
            if line.startswith(("-", "*", "\u2022", "\u2023")):
                if current_bullet:
                    bullets.append(current_bullet.strip())
                current_bullet = line.lstrip("-*\u2022\u2023").strip()
            elif current_bullet is not None:
                current_bullet = _append_continuation(current_bullet, line)
        if current_bullet:
            bullets.append(current_bullet.strip())

        company, title, start_date, end_date, location = _parse_experience_header(
            header_line
        )
        bullet_text = " ".join(bullets)
        detected = _extract_skills_from_text(bullet_text)
        entries.append(
            ExperienceEntry(
                company=company,
                title=title,
                start_date=start_date,
                end_date=end_date,
                location=location,
                bullets=bullets,
                detected_skills=detected,
                technologies=detected,
            )
        )
    return entries


def _is_external_metadata_line(line: str) -> bool:
    lowered = line.strip().lower()
    return lowered.startswith(("linkedin url:", "github url:", "external url:"))


def parse_resume_text_faithful(raw_text: str) -> ResumeProfile:
    lines = [
        line
        for line in _explode_inline_section_headers(raw_text)
        if not _is_external_metadata_line(line)
    ]
    sections = _split_sections(lines)

    email_match = _EMAIL_RE.search(raw_text)
    phone_match = _PHONE_RE.search(raw_text)
    linkedin_match = _LINKEDIN_RE.search(raw_text)
    name = next(
        (
            line.strip()
            for line in lines[:6]
            if line.strip() and "@" not in line and not _detect_header(line)
        ),
        None,
    )

    summary = _join_wrapped_lines(sections.get("summary", []))
    skills = _parse_skills_section(sections.get("skills", []))
    experiences = _parse_experience_section(sections.get("experience", []))
    education = [
        " ".join(line.split()).strip()
        for line in sections.get("education", [])
        if line.strip()
    ]
    certifications = [
        " ".join(line.split()).strip()
        for line in sections.get("certifications", [])
        if line.strip()
    ]

    return ResumeProfile(
        contact=ContactInfo(
            name=name,
            headline=_extract_headline(lines, name),
            email=email_match.group(0) if email_match else None,
            phone=phone_match.group(0) if phone_match else None,
            location=_extract_location(lines),
            linkedin_url=linkedin_match.group(0).rstrip(".,;") if linkedin_match else None,
        ),
        summary=summary,
        skills=skills,
        experiences=experiences,
        education=education,
        certifications=certifications,
    )


def _summary_is_suspicious(summary: str | None, source: ResumeProfile) -> bool:
    if not summary:
        return True
    lowered = summary.lower()
    markers = ("professional summary", "technical skills", "linkedin")
    if any(marker in lowered for marker in markers):
        return True
    for value in (
        source.contact.email,
        source.contact.phone,
        source.contact.name,
    ):
        if value and value.lower() in lowered:
            return True
    return False


def reconcile_ai_profile(raw_text: str, ai_profile: ResumeProfile) -> ResumeProfile:
    """Repair lossy AI output with exact source evidence from standard sections."""
    source = parse_resume_text_faithful(raw_text)

    contact = ai_profile.contact.model_copy()
    for field in ("name", "headline", "email", "phone", "location", "linkedin_url"):
        if not getattr(contact, field) and getattr(source.contact, field):
            setattr(contact, field, getattr(source.contact, field))

    summary = ai_profile.summary
    if source.summary and _summary_is_suspicious(summary, source):
        summary = source.summary
    elif source.summary and summary:
        # Parsing should never substantially expand or shrink a source summary.
        source_words = len(source.summary.split())
        ai_words = len(summary.split())
        if source_words and (ai_words < source_words * 0.75 or ai_words > source_words * 1.35):
            summary = source.summary

    # An explicit Technical Skills section is authoritative. Preserve versions
    # such as "Java 21", "Spring Boot 3.4", "OAuth 2.0", and product names.
    skills = source.skills or ai_profile.skills

    experiences = ai_profile.experiences
    if source.experiences:
        source_chars = sum(len(bullet) for exp in source.experiences for bullet in exp.bullets)
        ai_chars = sum(len(bullet) for exp in ai_profile.experiences for bullet in exp.bullets)
        same_shape = len(source.experiences) == len(ai_profile.experiences)
        if not ai_profile.experiences or not same_shape or ai_chars < source_chars * 0.9:
            experiences = source.experiences
        elif same_shape:
            repaired: list[ExperienceEntry] = []
            for ai_exp, source_exp in zip(ai_profile.experiences, source.experiences, strict=True):
                source_bullet_chars = sum(len(bullet) for bullet in source_exp.bullets)
                ai_bullet_chars = sum(len(bullet) for bullet in ai_exp.bullets)
                bullets = (
                    source_exp.bullets
                    if source_bullet_chars and ai_bullet_chars < source_bullet_chars * 0.9
                    else ai_exp.bullets
                )
                repaired.append(
                    ExperienceEntry(
                        company=source_exp.company or ai_exp.company,
                        title=source_exp.title or ai_exp.title,
                        start_date=source_exp.start_date or ai_exp.start_date,
                        end_date=source_exp.end_date or ai_exp.end_date,
                        location=source_exp.location or ai_exp.location,
                        bullets=bullets,
                        detected_skills=ai_exp.detected_skills or source_exp.detected_skills,
                        technologies=ai_exp.technologies or source_exp.technologies,
                    )
                )
            experiences = repaired

    return ResumeProfile(
        contact=contact,
        summary=summary,
        skills=skills,
        experiences=experiences,
        education=ai_profile.education or source.education,
        certifications=ai_profile.certifications or source.certifications,
    )
