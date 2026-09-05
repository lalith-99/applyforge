"""Tests for the heuristic job-description requirement parser."""

from app.jobs.parsing import parse_job_requirements

JD = """
We are looking for a Senior Backend Engineer to join our platform team.

Responsibilities:
- Design and build distributed backend services in Go.
- Operate Kafka-based event pipelines at scale.
- Collaborate with SRE on Kubernetes deployments.

Requirements:
- 5+ years of backend engineering experience.
- Strong experience with Go, PostgreSQL, and Docker.
- Bachelor's degree in Computer Science or related field.

Nice to have:
- Experience with Kubernetes and gRPC.
- AWS certification.

This role requires the ability to obtain a security clearance.
We are not able to sponsor employment visas for this position.
"""


def test_parse_extracts_required_and_preferred_skills() -> None:
    reqs = parse_job_requirements("Senior Backend Engineer", JD)
    required_names = {s.normalized_name for s in reqs.required_skills}
    preferred_names = {s.normalized_name for s in reqs.preferred_skills}

    assert "Go" in required_names
    assert "PostgreSQL" in required_names
    assert "Docker" in required_names
    # Kubernetes is mentioned in the Responsibilities section (before the
    # "Nice to have" marker), so it's correctly classified as required here.
    assert "Kubernetes" in required_names
    assert "gRPC" in preferred_names
    # A skill should not appear in both buckets.
    assert required_names.isdisjoint(preferred_names)


def test_parse_extracts_seniority_and_experience() -> None:
    reqs = parse_job_requirements("Senior Backend Engineer", JD)
    assert reqs.seniority == "Senior"
    assert reqs.required_experience_years == 5


def test_parse_extracts_responsibilities() -> None:
    reqs = parse_job_requirements("Senior Backend Engineer", JD)
    assert len(reqs.responsibilities) >= 2
    assert any("Kafka" in r for r in reqs.responsibilities)


def test_parse_extracts_clearance_and_work_authorization() -> None:
    reqs = parse_job_requirements("Senior Backend Engineer", JD)
    assert reqs.clearance_requirements is not None
    assert reqs.work_authorization_requirements is not None
    assert "sponsor" in reqs.work_authorization_requirements.lower()


def test_parse_extracts_education() -> None:
    reqs = parse_job_requirements("Senior Backend Engineer", JD)
    assert any("bachelor" in e.lower() for e in reqs.education_requirements)


def test_parse_handles_missing_sections_gracefully() -> None:
    reqs = parse_job_requirements("Engineer", "We use Go and PostgreSQL.")
    assert "Go" in reqs.keywords
    assert reqs.required_experience_years is None
