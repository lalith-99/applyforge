"""A curated dictionary of recognizable technology/skill keywords.

This powers the heuristic (non-LLM) resume and job-description parsers. It is
intentionally a plain, deterministic dictionary rather than a real NLP model —
see docs/AI_PIPELINE.md for why a heuristic provider is used until a real
AI_API_KEY is configured, and app/providers/README for the AIProvider seam
that will replace this with real model calls.
"""

from __future__ import annotations

SKILL_DICTIONARY: list[str] = [
    "Go", "Golang", "Python", "Java", "JavaScript", "TypeScript", "C++", "C#", "Ruby",
    "Rust", "Scala", "Kotlin", "Swift", "PHP", "Perl", "Elixir",
    "Node.js", "React", "React Native", "Vue", "Angular", "Next.js", "Django", "Flask",
    "FastAPI", "Spring", "Spring Boot", "Rails", ".NET", "ASP.NET", "GraphQL", "gRPC", "REST",
    "PostgreSQL", "MySQL", "SQLite", "MongoDB", "DynamoDB", "Redis", "Elasticsearch",
    "Cassandra", "SQL", "NoSQL",
    "Kafka", "RabbitMQ", "Amazon SQS", "Amazon SNS", "Google Pub/Sub",
    "Docker", "Kubernetes", "OpenShift", "Amazon ECS", "Terraform", "Ansible", "Helm", "Istio",
    "AWS", "Azure", "GCP", "Cloudflare",
  "Jenkins", "GitHub Actions", "GitLab CI", "CircleCI", "CI-CD",
    "Prometheus", "Grafana", "CloudWatch Metrics", "Datadog",
    "Spark", "Hadoop", "Airflow", "Pandas", "NumPy", "TensorFlow", "PyTorch",
    "Microservices", "WebSockets", "OAuth", "JWT", "Protobuf",
    "Linux", "Bash", "Git",
]

_LOWER_TO_CANONICAL = {s.lower(): s for s in SKILL_DICTIONARY}


def canonical_skills() -> list[str]:
    return list(SKILL_DICTIONARY)


def lookup(term: str) -> str | None:
    """Return the canonical spelling of a skill dictionary term, if known."""
    return _LOWER_TO_CANONICAL.get(term.strip().lower())
