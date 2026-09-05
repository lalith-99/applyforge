"""Quick Prep, Defend This Bullet, and learning-plan endpoints (§31, §32, §34).

Called by the Go API's internal/learning package. Not exposed to the browser.
"""

from __future__ import annotations

from fastapi import APIRouter

from app.learning.defend_bullet import defend_bullet as defend_bullet_fn
from app.learning.learning_plan import generate_learning_plan
from app.learning.models import (
    DefendBulletRequest,
    DefendBulletResponse,
    LearningPlanRequest,
    LearningPlanResponse,
    QuickPrepModule,
    QuickPrepRequest,
)
from app.learning.quick_prep import generate_quick_prep

router = APIRouter(prefix="/v1/learning", tags=["learning"])


@router.post("/quick-prep", response_model=QuickPrepModule)
def quick_prep(request: QuickPrepRequest) -> QuickPrepModule:
    return generate_quick_prep(request)


@router.post("/defend-bullet", response_model=DefendBulletResponse)
def defend_bullet_endpoint(request: DefendBulletRequest) -> DefendBulletResponse:
    return defend_bullet_fn(request)


@router.post("/learning-plan", response_model=LearningPlanResponse)
def learning_plan(request: LearningPlanRequest) -> LearningPlanResponse:
    return generate_learning_plan(request)
