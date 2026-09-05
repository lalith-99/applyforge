"""Heuristic (non-LLM) job-description requirement extraction.

Deterministic stand-in for the AI-driven JD extraction described in
MASTER_REQUIREMENTS.md §17, for the same reasons as app/resume/parsing.py:
functional end-to-end pipeline without an AI_API_KEY configured.
"""

from __future__ import annotations

import re

from app.core.skills_dictionary import canonical_skills
from app.jobs.models import JobRequirements, SkillRequirement

_PREFERRED_MARKERS = re.compile(
    r"(nice.to.have|preferred|bonus|a plus|good to have)", re.IGNORECASE
)
_REQUIRED_MARKERS = re.compile(
    r"(requirements?|must.have|required|minimum qualifications|what you.ll need|who you are)",
    re.IGNORECASE,
)
_RESPONSIBILITY_MARKERS = re.compile(
    r"(responsibilities|what you.ll do|the role|in this role)", re.IGNORECASE
)
_SENIORITY_RE = re.compile(
    r"\b(principal|staff|senior|sr\.?|lead|junior|jr\.?|entry.level|intern)\b", re.IGNORECASE
)
_EXPERIENCE_RE = re.compile(r"(\d+)\s*\+?\s*years?", re.IGNORECASE)
_EDUCATION_RE = re.compile(
    r"(bachelor.s degree|master.s degree|ph\.?d\.?|b\.?s\.?c?\.?\s+in|m\.?s\.?\s+in)", re.IGNORECASE
)
_CLEARANCE_RE = re.compile(
    r"(security clearance|top secret|ts/sci|secret clearance)", re.IGNORECASE
)
_WORK_AUTH_RE = re.compile(
    r"([^.\n]*\b(sponsor(ship)?|authorized to work|work authorization|visa)\b[^.\n]*)",
    re.IGNORECASE,
)
_BULLET_RE = re.compile(r"^\s*[-*\u2022\u2023]\s*(.+)$")


def _find_skills(text: str) -> list[str]:
    lowered = text.lower()
    found = []
    for skill in canonical_skills():
        pattern = r"(?<![\w+#.-])" + re.escape(skill.lower()) + r"(?![\w+#-])"
        if re.search(pattern, lowered):
            found.append(skill)
    return found


def _split_required_preferred(description: str) -> tuple[str, str]:
    """Best-effort split of the description into required vs preferred text
    blocks based on section markers; falls back to treating everything as
    required if no preferred-section marker is found."""
    preferred_match = _PREFERRED_MARKERS.search(description)
    if not preferred_match:
        return description, ""
    return description[: preferred_match.start()], description[preferred_match.start() :]


def _extract_seniority(title: str, description: str) -> str | None:
    match = _SENIORITY_RE.search(title) or _SENIORITY_RE.search(description[:500])
    return match.group(1).title() if match else None


def _split_into_sentences(text: str) -> list[str]:
    """Crude sentence splitter used when a description has no bullet
    markers at all (plain-prose postings)."""
    parts = re.split(r"(?<=[.!?])\s+", text)
    return [p.strip() for p in parts if len(p.strip()) > 25]


def _extract_bullets(section: str) -> list[str]:
    lines = [line.strip() for line in section.splitlines() if line.strip()]
    bullets = []
    for line in lines:
        bullet_match = _BULLET_RE.match(line)
        if bullet_match:
            bullets.append(bullet_match.group(1).strip())
    return bullets


def _extract_responsibilities(description: str) -> list[str]:
    marker = _RESPONSIBILITY_MARKERS.search(description)
    if marker:
        section = description[marker.end() : marker.end() + 1500]
        # Stop at the next likely section header.
        next_section = _PREFERRED_MARKERS.search(section) or _REQUIRED_MARKERS.search(section)
        if next_section:
            section = section[: next_section.start()]
    else:
        # No explicit "Responsibilities"/"What you'll do" header - many real
        # postings still list duties, just without that exact heading. Fall
        # back to the whole description instead of returning nothing.
        section = description

    bullets = _extract_bullets(section)
    if bullets:
        return bullets[:10]

    # No bullet markers either (plain-prose posting) - sentence-split so we
    # still surface something instead of an empty responsibilities list.
    return _split_into_sentences(section)[:10]


def parse_job_requirements(title: str, description: str) -> JobRequirements:
    required_text, preferred_text = _split_required_preferred(description)

    required_skill_names = _find_skills(required_text)
    preferred_skill_names = [
        s for s in _find_skills(preferred_text) if s not in required_skill_names
    ]

    required_skills = [
        SkillRequirement(normalized_name=s, original_text=s, importance="required")
        for s in required_skill_names
    ]
    preferred_skills = [
        SkillRequirement(normalized_name=s, original_text=s, importance="preferred")
        for s in preferred_skill_names
    ]

    experience_match = _EXPERIENCE_RE.search(description)
    education_matches = _EDUCATION_RE.findall(description)
    clearance_match = _CLEARANCE_RE.search(description)
    work_auth_match = _WORK_AUTH_RE.search(description)

    all_keywords = sorted(set(required_skill_names) | set(preferred_skill_names))

    return JobRequirements(
        role_family=title.strip() or None,
        normalized_title=title.strip() or None,
        seniority=_extract_seniority(title, description),
        required_skills=required_skills,
        preferred_skills=preferred_skills,
        required_experience_years=int(experience_match.group(1)) if experience_match else None,
        responsibilities=_extract_responsibilities(description),
        domains=[],
        education_requirements=list(
            dict.fromkeys(m if isinstance(m, str) else m[0] for m in education_matches)
        ),
        certifications=[],
        clearance_requirements=clearance_match.group(0) if clearance_match else None,
        work_authorization_requirements=work_auth_match.group(1).strip()
        if work_auth_match
        else None,
        keywords=all_keywords,
    )


def parse_job_requirements_ai(title: str, description: str) -> JobRequirements:
    """Real LLM-backed JD requirement extraction. Raises AIProviderError
    (see app/providers/openai_provider.py) on any failure — callers must
    fall back to parse_job_requirements() above."""
    from app.providers.openai_provider import structured_completion

    system = (
        "You are an expert technical recruiter. Extract structured requirements from this job posting "
        "into the exact schema. Classify each skill as required or preferred using the language in the "
        "post (e.g. 'must have'/'required' vs 'nice to have'/'preferred'/'bonus'). Only include skills "
        "or requirements explicitly stated or clearly implied by the text — never invent requirements "
        "not present in the posting. normalized_name should be the common/canonical form of the skill "
        "name (e.g. 'JS' -> 'JavaScript')."
    )
    user = f"Job title: {title}\n\nJob description:\n{description}"
    return structured_completion(system, user, JobRequirements)
