"""Regression tests for source-faithful resume parsing."""

# ruff: noqa: E501

from app.resume.faithful import parse_resume_text_faithful, reconcile_ai_profile
from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile


RAW_RESUME = """Vandana Bharatha
Java Software Engineer | Spring Boot | Microservices | AWS | Kafka
Maryland, USA | (806) 451-9997 | vandanabharatha16@gmail.com | LinkedIn
PROFESSIONAL SUMMARY
Java Software Engineer with 5+ years of experience designing, developing, and modernizing enterprise backend and full-stack
applications across healthcare, fintech, and financial-services environments.
TECHNICAL SKILLS
Languages: Java 8, Java 21, Python, JavaScript, TypeScript, SQL, Bash
Backend: Spring Boot 3.4, J2EE, REST APIs, Microservices, Spring Security, JPA, Hibernate, Service Layer Architecture, Design
Patterns
Databases & Messaging: Oracle, PostgreSQL, MySQL, MongoDB, Redis, Apache Kafka, SQL Optimization, Index Optimization,
Asynchronous Processing
Cloud & DevOps: AWS, Amazon ECS, Amazon S3, Route 53, Lambda, Kubernetes, Docker, Terraform, Jenkins, Maven, CI/CD
Frontend: Angular 19, React.js, HTML5, CSS3, Responsive UI, Form Validation, Section 508 Accessibility
Security: OAuth 2.0, SAML SSO, RBAC, PCI DSS
Testing & Tools: JUnit, Selenium, Jest, TDD, Git, Jira, Agile/Scrum, SAFe Agile, Code Reviews
AI / GenAI: RAG, LangChain, Amazon Bedrock, OpenAI APIs, LLMs, Semantic Retrieval, Claude Code
PROFESSIONAL EXPERIENCE
Centers for Medicare & Medicaid Services (CMS) | Software Development Engineer | Aug 2024 – Present / USA
• Develop and modernize enterprise healthcare applications supporting NPPES provider-profile, validation, and administrative-
review workflows using Java 21, Spring Boot, REST APIs, JPA, Oracle, and Angular 19.
• Migrate legacy J2EE components to Java 21 and Spring Boot 3.4 microservices, improving application scalability by 40% and
reducing API response times by up to 30%.
Thoughtworks | Software Developer | May 2023 - Jul 2024 / USA
• Developed and maintained Java Spring Boot REST APIs across eight merchant-facing fintech payment services, improving
backend reliability, transaction monitoring, and release stability.
EDUCATION
Master of Science in Computer Science
Jan 2022 – Aug 2023 | Lubbock, Texas Texas Tech University, USA
"""


def test_faithful_parser_preserves_exact_skills_and_versions() -> None:
    profile = parse_resume_text_faithful(RAW_RESUME)

    assert profile.contact.name == "Vandana Bharatha"
    assert profile.contact.headline == (
        "Java Software Engineer | Spring Boot | Microservices | AWS | Kafka"
    )
    assert profile.contact.location == "Maryland, USA"

    assert "Java 8" in profile.skills
    assert "Java 21" in profile.skills
    assert "Spring Boot 3.4" in profile.skills
    assert "Amazon S3" in profile.skills
    assert "Route 53" in profile.skills
    assert "OAuth 2.0" in profile.skills
    assert "JUnit" in profile.skills
    assert "Java" not in profile.skills


def test_faithful_parser_joins_wrapped_experience_bullets() -> None:
    profile = parse_resume_text_faithful(RAW_RESUME)

    assert len(profile.experiences) == 2
    cms = profile.experiences[0]
    assert cms.company == "Centers for Medicare & Medicaid Services (CMS)"
    assert cms.title == "Software Development Engineer"
    assert cms.start_date == "Aug 2024"
    assert cms.end_date == "Present"
    assert cms.location == "USA"
    assert len(cms.bullets) == 2
    assert "administrative-review workflows" in cms.bullets[0]
    assert cms.bullets[0].endswith("Angular 19.")
    assert cms.bullets[1].endswith("30%.")


def test_faithful_parser_keeps_header_out_of_summary() -> None:
    profile = parse_resume_text_faithful(RAW_RESUME)

    assert profile.summary is not None
    assert profile.summary.startswith("Java Software Engineer with 5+ years")
    assert "PROFESSIONAL SUMMARY" not in profile.summary
    assert "vandanabharatha16@gmail.com" not in profile.summary


def test_reconcile_repairs_lossy_ai_output() -> None:
    degraded = ResumeProfile(
        contact=ContactInfo(name="Vandana Bharatha"),
        summary=(
            "Vandana Bharatha | vandanabharatha16@gmail.com | LinkedIn "
            "PROFESSIONAL SUMMARY Java engineer."
        ),
        skills=["Java", "Spring", "AWS"],
        experiences=[
            ExperienceEntry(
                company="Software Development Engineer",
                title="Centers for Medicare & Medicaid Services (CMS)",
                bullets=["Develop and modernize enterprise healthcare applications supporting"],
            ),
            ExperienceEntry(
                company="Thoughtworks",
                title="Software Developer",
                bullets=["Developed and maintained Java Spring Boot REST APIs across"],
            ),
        ],
    )

    repaired = reconcile_ai_profile(RAW_RESUME, degraded)

    assert repaired.summary is not None
    assert repaired.summary.startswith("Java Software Engineer with 5+ years")
    assert "Java 21" in repaired.skills
    assert repaired.experiences[0].company == (
        "Centers for Medicare & Medicaid Services (CMS)"
    )
    assert repaired.experiences[0].title == "Software Development Engineer"
    assert repaired.experiences[0].bullets[0].endswith("Angular 19.")
