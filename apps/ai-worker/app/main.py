"""ApplyForge AI/document worker service entrypoint."""

from fastapi import FastAPI

from app.api.health import router as health_router

app = FastAPI(title="ApplyForge AI Worker", version="0.1.0")

app.include_router(health_router)
