"""ApplyForge AI/document worker service entrypoint."""

from __future__ import annotations

import os
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.candidates import router as candidates_router
from app.api.documents import router as documents_router
from app.api.embeddings import router as embeddings_router
from app.api.health import router as health_router
from app.api.jobs import router as jobs_router
from app.api.learning import router as learning_router
from app.api.resumes import router as resumes_router
from app.api.tailoring import router as tailoring_router


def validate_runtime_config() -> None:
    environment = os.environ.get("ENVIRONMENT", "development").strip().lower()
    if environment != "production":
        return

    if not os.environ.get("OPENAI_API_KEY", "").strip():
        raise RuntimeError(
            "OPENAI_API_KEY is required in production because semantic embeddings "
            "cannot fall back to deterministic heuristics"
        )


@asynccontextmanager
async def lifespan(_: FastAPI):
    validate_runtime_config()
    yield


app = FastAPI(title="ApplyForge AI Worker", version="0.1.0", lifespan=lifespan)

app.include_router(health_router)
app.include_router(resumes_router)
app.include_router(jobs_router)
app.include_router(tailoring_router)
app.include_router(learning_router)
app.include_router(documents_router)
app.include_router(embeddings_router)
app.include_router(candidates_router)
