"""Tests for PDF/DOCX resume document generation."""

import fitz
from fastapi.testclient import TestClient

from app.documents.generator import _sanitize_for_pdf, render_docx, render_pdf
from app.main import app
from app.resume.models import ContactInfo, ExperienceEntry, ResumeProfile

client = TestClient(app)


def _sample_profile() -> ResumeProfile:
    return ResumeProfile(
        contact=ContactInfo(
            name="Ada Lovelace",
            headline="Backend Engineer | Distributed Systems | AWS | Kafka",
            email="ada@example.com",
            phone="555-0100",
            location="NYC",
            linkedin_url="https://www.linkedin.com/in/ada-lovelace/",
        ),
        summary="Backend engineer with 6 years of experience.",
        skills=["Go", "PostgreSQL", "Kafka", "AWS", "Docker", "Kubernetes"],
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


def _realistic_one_page_profile() -> ResumeProfile:
    skills = [
        "Java 8",
        "Java 21",
        "Python",
        "JavaScript",
        "TypeScript",
        "SQL",
        "Bash",
        "Spring Boot 3.4",
        "J2EE",
        "REST APIs",
        "Microservices",
        "Spring Security",
        "JPA",
        "Hibernate",
        "Service Layer Architecture",
        "Design Patterns",
        "Oracle",
        "PostgreSQL",
        "MySQL",
        "MongoDB",
        "Redis",
        "Apache Kafka",
        "SQL Optimization",
        "Index Optimization",
        "Asynchronous Processing",
        "AWS",
        "Amazon ECS",
        "Amazon S3",
        "Route 53",
        "Lambda",
        "Kubernetes",
        "Docker",
        "Terraform",
        "Jenkins",
        "Maven",
        "CI/CD",
        "Angular 19",
        "React.js",
        "HTML5",
        "CSS3",
        "Responsive UI",
        "Form Validation",
        "Section 508 Accessibility",
        "OAuth 2.0",
        "SAML SSO",
        "RBAC",
        "PCI DSS",
        "JUnit",
        "Selenium",
        "Jest",
        "TDD",
        "Git",
        "Jira",
        "Agile/Scrum",
        "SAFe Agile",
        "Code Reviews",
        "RAG",
        "LangChain",
        "Amazon Bedrock",
        "OpenAI APIs",
        "LLMs",
        "Semantic Retrieval",
        "Claude Code",
    ]
    return ResumeProfile(
        contact=ContactInfo(
            name="Jordan Rivera",
            headline="Java Software Engineer | Spring Boot | Microservices | AWS | Kafka",
            email="jordan@example.com",
            phone="555-0100",
            location="Maryland, USA",
            linkedin_url="https://www.linkedin.com/in/jordan-rivera/",
        ),
        summary=(
            "Java Software Engineer with 5+ years of experience designing, developing, and "
            "modernizing enterprise backend and full-stack applications across healthcare, "
            "fintech, and financial-services environments. Strong experience with Java, "
            "Spring Boot, REST APIs, microservices, Kafka, Redis, AWS, Docker, and Kubernetes."
        ),
        skills=skills,
        experiences=[
            ExperienceEntry(
                company="Healthcare Program",
                title="Software Development Engineer",
                start_date="Aug 2024",
                end_date="Present",
                location="USA",
                bullets=[
                    ("Develop and modernize enterprise applications using Java 21, Spring Boot, "
                     "REST APIs, JPA, Oracle, and Angular."),
                    ("Migrate legacy J2EE components to Java 21 and Spring Boot microservices, "
                     "improving scalability and API response times."),
                    ("Design REST APIs and reusable service-layer components using Spring Boot, "
                     "JPA, and Oracle."),
                    ("Build Angular interfaces integrated with Spring Boot APIs and improve "
                     "accessibility workflows."),
                    ("Integrate Spring Boot microservices with Amazon ECS, Amazon S3, and Route 53 "
                     "for cloud-ready deployments."),
                    ("Build internal retrieval-augmented search using LangChain, Bedrock, "
                     "OpenAI APIs, and semantic retrieval."),
                    ("Support dependency upgrades, secure refactoring, and vulnerability "
                     "remediation."),
                ],
            ),
            ExperienceEntry(
                company="Fintech Consultancy",
                title="Software Developer",
                start_date="May 2023",
                end_date="Jul 2024",
                location="USA",
                bullets=[
                    ("Developed Java Spring Boot REST APIs across merchant-facing fintech payment "
                     "services."),
                    ("Migrated monolithic payment modules to AWS- and Kubernetes-based "
                     "microservices."),
                    ("Reduced API latency through Redis caching, Kafka asynchronous processing, "
                     "and service optimizations."),
                    ("Designed scalable backend services for distributed payment-processing "
                     "workflows."),
                    ("Built Python risk-scoring workflows to improve suspicious-activity "
                     "prioritization."),
                    "Increased automated test coverage using Selenium and Jest.",
                ],
            ),
            ExperienceEntry(
                company="Enterprise Technology Company",
                title="Software Engineer",
                start_date="Jan 2020",
                end_date="Dec 2021",
                location="India",
                bullets=[
                    ("Developed Java Spring Boot REST APIs and optimized SQL queries and database "
                     "indexes."),
                    ("Modernized legacy application modules into independently deployable "
                     "Spring Boot microservices."),
                    ("Designed persistence and service-layer components using Hibernate, SQL, "
                     "and MySQL."),
                    ("Built Jenkins CI/CD pipelines using Maven, JUnit, Git, Docker, automated "
                     "testing, and deployment stages."),
                    "Implemented OAuth 2.0, SAML SSO, and role-based access control.",
                ],
            ),
        ],
        education=[
            "Master of Science in Computer Science",
            "Jan 2022 - Aug 2023 | Texas Tech University | Lubbock, Texas, USA",
        ],
    )


def test_render_pdf_produces_valid_pdf_bytes() -> None:
    data = render_pdf(_sample_profile())
    assert data.startswith(b"%PDF")
    assert len(data) > 100


def test_render_pdf_preserves_clickable_linkedin() -> None:
    data = render_pdf(_sample_profile())
    with fitz.open(stream=data, filetype="pdf") as doc:
        uris = [link.get("uri") for page in doc for link in page.get_links()]
    assert "https://www.linkedin.com/in/ada-lovelace/" in uris


def test_render_pdf_realistic_resume_stays_one_page() -> None:
    data = render_pdf(_realistic_one_page_profile())
    with fitz.open(stream=data, filetype="pdf") as doc:
        assert doc.page_count == 1


def test_render_docx_produces_valid_zip_bytes() -> None:
    data = render_docx(_sample_profile())
    assert data[:2] == b"PK"
    assert len(data) > 100


def test_render_pdf_handles_minimal_profile() -> None:
    minimal = ResumeProfile()
    data = render_pdf(minimal)
    assert data.startswith(b"%PDF")


def test_render_pdf_handles_ligatures_and_smart_punctuation() -> None:
    profile = ResumeProfile(
        summary="Certi\ufb01cations and Technical Skills: \u201cquoted\u201d \u2013 en \u2014 em",
    )
    data = render_pdf(profile)
    assert data.startswith(b"%PDF")


def test_sanitize_for_pdf_decomposes_fi_ligature() -> None:
    assert _sanitize_for_pdf("Certi\ufb01cations") == "Certifications"


def test_sanitize_for_pdf_preserves_smart_punctuation_via_cp1252() -> None:
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
