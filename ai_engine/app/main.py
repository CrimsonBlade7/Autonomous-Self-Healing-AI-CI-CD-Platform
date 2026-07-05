from fastapi import FastAPI

from app.core.config import settings  # noqa: F401 — imported to validate env on startup

app = FastAPI(
    title="AI Engine",
    description="AI/Data Processing service for the Self-Healing CI/CD Platform",
    version="0.1.0",
)


@app.get("/health")
async def health_check():
    """Liveness probe — returns 200 when the service is running."""
    return {"status": "ok"}


# Phase 5: POST /analyze endpoint
# Accepts a job-failure event from the Go orchestrator and enqueues a Celery task.
# Returns 202 Accepted with a job_id the orchestrator can use to poll.


# Phase 5: GET /result/{job_id}
# Go orchestrator polls this endpoint to retrieve the generated patch once ready.
