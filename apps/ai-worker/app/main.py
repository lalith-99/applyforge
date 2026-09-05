"""ApplyForge AI/document worker service entrypoint."""

from fastapi import FastAPI

from app.api.health import router as health_router
from app.api.jobs import router as jobs_router
from app.api.learning import router as learning_router
from app.api.resumes import router as resumes_router
from app.api.tailoring import router as tailoring_router

app = FastAPI(title="ApplyForge AI Worker", version="0.1.0")

app.include_router(health_router)
app.include_router(resumes_router)
app.include_router(jobs_router)
app.include_router(tailoring_router)
app.include_router(learning_router)
