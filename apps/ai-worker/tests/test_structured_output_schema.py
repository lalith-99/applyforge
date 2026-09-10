"""Regression tests for provider-facing OpenAI Structured Outputs schemas.

These tests exercise the same strict-schema converter used by the OpenAI Python
SDK. They specifically guard against arbitrary-key maps, which the API rejects
before model inference and which previously caused MAX_MATCH to silently fall
back to skills-only heuristic output.
"""

from __future__ import annotations

from typing import Any

from openai.lib._pydantic import to_strict_json_schema

from app.resume.models import ResumeProfile
from app.tailoring.enrichment import TailoringEnrichmentResponse
from app.tailoring.models import ExperienceSupportResponse, TailoringResponse, TailoringSuggestion
from app.tailoring.support_bullets import _OneSupport, _ThreeSupports, _TwoSupports


def _assert_no_open_objects(node: object, path: tuple[str, ...] = ()) -> None:
    if isinstance(node, list):
        for index, item in enumerate(node):
            _assert_no_open_objects(item, (*path, str(index)))
        return
    if not isinstance(node, dict):
        return

    if node.get("type") == "object":
        additional = node.get("additionalProperties")
        assert additional is False, (
            f"OpenAI strict schema contains an arbitrary/open object at {path}: "
            f"additionalProperties={additional!r}"
        )
        properties = node.get("properties", {})
        if isinstance(properties, dict):
            assert set(node.get("required", [])) == set(properties), (
                f"OpenAI strict object at {path} does not require every property"
            )

    for key, value in node.items():
        _assert_no_open_objects(value, (*path, str(key)))


def _strict_schema(model: type[Any]) -> dict[str, Any]:
    schema = to_strict_json_schema(model)
    _assert_no_open_objects(schema)
    return schema


def _find_property_schemas(node: object, property_name: str) -> list[dict[str, Any]]:
    found: list[dict[str, Any]] = []
    if isinstance(node, list):
        for item in node:
            found.extend(_find_property_schemas(item, property_name))
        return found
    if not isinstance(node, dict):
        return found

    properties = node.get("properties")
    if isinstance(properties, dict):
        candidate = properties.get(property_name)
        if isinstance(candidate, dict):
            found.append(candidate)
    for value in node.values():
        found.extend(_find_property_schemas(value, property_name))
    return found


def test_all_tailoring_openai_response_schemas_have_no_dynamic_maps() -> None:
    models = (
        TailoringResponse,
        TailoringEnrichmentResponse,
        ExperienceSupportResponse,
        _OneSupport,
        _TwoSupports,
        _ThreeSupports,
    )
    for model in models:
        schema = _strict_schema(model)
        category_fields = _find_property_schemas(schema, "skill_categories")
        assert category_fields, f"expected skill_categories in {model.__name__} schema"
        assert all(field.get("type") == "array" for field in category_fields)


def test_resume_parse_openai_schema_has_no_dynamic_map() -> None:
    schema = _strict_schema(ResumeProfile)
    category_fields = _find_property_schemas(schema, "skill_categories")
    assert category_fields
    assert all(field.get("type") == "array" for field in category_fields)


def test_tailoring_skill_category_wire_array_round_trips_to_domain_map() -> None:
    suggestion = TailoringSuggestion.model_validate(
        {
            "section": "skills",
            "original_text": None,
            "suggested_text": "Add Prompt Engineering",
            "requirements_addressed": ["Prompt Engineering"],
            "skills_added": ["Prompt Engineering"],
            "keywords_added": ["Prompt Engineering"],
            "skill_categories": [
                {"skill": "Prompt Engineering", "category": "AI / GenAI"}
            ],
            "operation": "REWRITE",
            "target_company": None,
            "target_title": None,
            "source": "AI_SUGGESTED",
            "reason": "Required by target role.",
            "confidence": 0.8,
            "risk_level": "HIGH",
        }
    )

    assert suggestion.skill_categories == {"Prompt Engineering": "AI / GenAI"}
    assert suggestion.model_dump()["skill_categories"] == {
        "Prompt Engineering": "AI / GenAI"
    }


def test_resume_skill_category_wire_array_round_trips_to_domain_map() -> None:
    profile = ResumeProfile.model_validate(
        {
            "skill_categories": [
                {"skill": "Prompt Engineering", "category": "AI / GenAI"}
            ]
        }
    )
    assert profile.skill_categories == {"Prompt Engineering": "AI / GenAI"}
    assert profile.model_dump()["skill_categories"] == {
        "Prompt Engineering": "AI / GenAI"
    }
