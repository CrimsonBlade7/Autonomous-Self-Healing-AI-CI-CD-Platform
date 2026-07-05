from celery import Celery

from app.core.config import settings

# ---------------------------------------------------------------------------
# Celery application
# ---------------------------------------------------------------------------
# The broker (Redis) is the queue Celery uses to receive task messages.
# The backend (also Redis) stores task results so the FastAPI /result endpoint
# can retrieve them after the worker finishes.
# ---------------------------------------------------------------------------
celery_app = Celery(
    "ai_engine",
    broker=settings.redis_url,
    backend=settings.redis_url,
)

celery_app.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
)


# ---------------------------------------------------------------------------
# Phase 5: Tasks
#
# analyze_and_heal(job_id, logs, repo_context)
#   Chains: log_analyzer -> rag_pipeline -> patch_gen
#   Stores the resulting patch as the Celery task result so the Go orchestrator
#   can retrieve it via GET /result/{job_id}.
# ---------------------------------------------------------------------------
