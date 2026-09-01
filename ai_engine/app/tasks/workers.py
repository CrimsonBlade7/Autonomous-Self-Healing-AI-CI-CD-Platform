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


@celery_app.task(name="analyze_log")
def analyze_log(raw_log: str) -> dict:
    """Parse a CI log and store the sanitized response in the Redis backend."""
    from app.services.log_response import create_log_response

    return create_log_response(raw_log)


# ---------------------------------------------------------------------------
# Phase 5: Tasks
#
# analyze_and_heal(job_id, logs, repo_context)
#   Chains: log_analyzer -> rag_pipeline -> patch_gen
#   Stores the resulting patch as the Celery task result so the Go orchestrator
#   can retrieve it via GET /result/{job_id}.
# ---------------------------------------------------------------------------


@celery_app.task(name="analyze_and_generate_tests")
def analyze_and_generate_tests(job_id: str, raw_log: str, source: str = "ci") -> dict:
    """Parse *raw_log*, generate a fixture + pytest stub, and return the package.

    Result is stored in the Celery backend so the orchestrator can retrieve it
    via GET /result/{job_id} once the task completes.
    """
    from app.services.test_gen import generate_tests

    package = generate_tests(raw_log, source=source)
    return {
        "job_id": job_id,
        "status": "completed",
        "fixture_slug": package["fixture_slug"],
        "expected_json": package["expected_json"],
        "metadata_json": package["metadata_json"],
        "test_stub": package["test_stub"],
        "documentation": package["documentation"],
        "saved_to_disk": package["saved_to_disk"],
        "fixture_path": package["fixture_path"],
    }
