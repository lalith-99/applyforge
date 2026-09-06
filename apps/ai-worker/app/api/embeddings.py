"""Semantic embeddings endpoint - used for both job and candidate profile
embeddings (Phase E/F: pgvector-backed semantic retrieval). Generic on
purpose (takes arbitrary text) so the same endpoint serves both callers.
"""

from __future__ import annotations

import os

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from app.providers.openai_provider import AIProviderError, embed_text, is_configured

router = APIRouter(prefix="/v1/embeddings", tags=["embeddings"])


class EmbedRequest(BaseModel):
    text: str


class EmbedResponse(BaseModel):
    embedding: list[float]
    model: str
    dimensions: int


@router.post("", response_model=EmbedResponse)
def embed(request: EmbedRequest) -> EmbedResponse:
    if not is_configured():
        raise HTTPException(
            status_code=503, detail="embeddings require OPENAI_API_KEY to be configured"
        )
    if not request.text.strip():
        raise HTTPException(status_code=400, detail="text must not be empty")

    try:
        vector = embed_text(request.text)
    except AIProviderError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    model = os.environ.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    return EmbedResponse(embedding=vector, model=model, dimensions=len(vector))
