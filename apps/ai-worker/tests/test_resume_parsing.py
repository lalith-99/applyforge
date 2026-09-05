"""Tests for the heuristic resume parser."""

from app.resume.parsing import parse_resume_text

SAMPLE_RESUME = """Jordan Rivera
jordan.rivera@example.com | (555) 123-4567

SUMMARY
Backend engineer with 6 years of experience building distributed systems.

SKILLS
Go, Kafka, PostgreSQL, Docker, Kubernetes, AWS

EXPERIENCE
Senior Backend Engineer, Acme Corp (Jan 2021 - Present)
- Built highly concurrent Go services processing telecom workloads through Kafka pipelines.
- Migrated deployments from Docker Compose to Kubernetes.

Backend Engineer, Beta Inc (Jun 2018 - Dec 2020)
- Designed PostgreSQL schemas for a multi-tenant billing system.

EDUCATION
B.S. Computer Science, State University

CERTIFICATIONS
AWS Certified Solutions Architect
"""


def test_parse_resume_extracts_contact_info() -> None:
    profile = parse_resume_text(SAMPLE_RESUME)
    assert profile.contact.name == "Jordan Rivera"
    assert profile.contact.email == "jordan.rivera@example.com"
    assert profile.contact.phone is not None


def test_parse_resume_extracts_skills() -> None:
    profile = parse_resume_text(SAMPLE_RESUME)
    assert "Go" in profile.skills
    assert "Kafka" in profile.skills
    assert "PostgreSQL" in profile.skills
    assert "Kubernetes" in profile.skills


def test_parse_resume_extracts_experiences() -> None:
    profile = parse_resume_text(SAMPLE_RESUME)
    assert len(profile.experiences) == 2

    first = profile.experiences[0]
    assert first.title == "Senior Backend Engineer"
    assert first.company == "Acme Corp"
    assert first.start_date is not None
    assert "Kafka" in first.detected_skills
    assert len(first.bullets) == 2


def test_parse_resume_extracts_education_and_certifications() -> None:
    profile = parse_resume_text(SAMPLE_RESUME)
    assert any("Computer Science" in line for line in profile.education)
    assert any("AWS Certified" in line for line in profile.certifications)


def test_parse_resume_handles_empty_input() -> None:
    profile = parse_resume_text("")
    assert profile.skills == []
    assert profile.experiences == []
