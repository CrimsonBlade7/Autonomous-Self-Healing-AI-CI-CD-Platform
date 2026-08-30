from __future__ import annotations

import re
import uuid
from typing import Literal

from fastapi import BackgroundTasks, FastAPI, HTTPException, Request
from pydantic import BaseModel, Field

from app.core.config import settings  # noqa: F401 — imported to validate env on startup
from app.services.log_analyzer import parse_log


class AnalyzeRequest(BaseModel):
    """Incoming payload for log analysis requests."""

    raw_log: str = Field(..., min_length=1)
    source: Literal["ci", "user"] = "ci"


class ParsedLogResponse(BaseModel):
    """User-facing structured log analysis response."""

    error_type: str
    failing_test: str
    stack_trace_lines: list[str]
    error_signature: str
    language: Literal["python", "node", "go", "java", "rust", "unknown"]
    framework: str
    confidence: float
    root_cause_message: str
    parser_version: str
    fallback_reason: str


class GenerateTestsRequest(BaseModel):
    """Incoming payload for test-generation requests."""

    raw_log: str = Field(..., min_length=1)
    source: Literal["ci", "user"] = "ci"
    fixture_name: str | None = Field(None, min_length=1, max_length=80)


class GeneratedTestsResponse(BaseModel):
    """Response returned by POST /generate-tests."""

    fixture_slug: str
    expected_json: dict
    metadata_json: dict
    test_stub: str
    documentation: str
    saved_to_disk: bool
    fixture_path: str


class AnalyzeAcceptedResponse(BaseModel):
    """Acknowledgement returned when analysis is accepted."""

    job_id: str
    status: Literal["accepted"]


class AnalyzeResultResponse(BaseModel):
    """Result payload returned for completed analysis jobs."""

    job_id: str
    status: Literal["completed"]
    result: ParsedLogResponse


_ABS_PATH_RE = re.compile(r"(?:[A-Za-z]:)?/(?:[^\s/:]+/)+[^\s:]+")
_UUID_RE = re.compile(
    r"\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}\b"
)
_LONG_HEX_RE = re.compile(r"\b[0-9a-fA-F]{12,}\b")
_TIME_RE = re.compile(r"\b\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b")
_SECRET_RE = re.compile(r"(?i)\b(?:api[_-]?key|token|password|secret)\s*[:=]\s*\S+")

_JOB_RESULTS: dict[str, AnalyzeResultResponse] = {}


def _redact_text(value: str) -> str:
    """Mask high-risk and high-variance tokens from user-facing log text."""
    value = _ABS_PATH_RE.sub("<path>", value)
    value = _UUID_RE.sub("<uuid>", value)
    value = _LONG_HEX_RE.sub("<hash>", value)
    value = _TIME_RE.sub("<time>", value)
    value = _SECRET_RE.sub("<secret>", value)
    return value


def _infer_language_and_framework(raw_log: str) -> tuple[str, str, float]:
    """Infer ecosystem metadata using simple deterministic signal matching."""
    lowered = raw_log.lower()
    if any(token in lowered for token in ("traceback", "pytest", "assertionerror", "modulenotfounderror")):
        return "python", "pytest", 0.88
    if any(token in lowered for token in ("jest", "vitest", "npm err!", "typeerror:")):
        return "node", "jest", 0.78
    if any(token in lowered for token in ("--- fail:", "panic:", "go test")):
        return "go", "go test", 0.82
    if any(token in lowered for token in ("caused by:", "exception in thread", "mvn test", "gradle")):
        return "java", "maven", 0.8
    if any(token in lowered for token in ("cargo test", "panicked at", "error[e")):
        return "rust", "cargo", 0.81
    return "unknown", "unknown", 0.35


def _to_user_response(raw_log: str) -> ParsedLogResponse:
    """Convert parser output into the full user-facing schema."""
    parsed = parse_log(raw_log)
    language, framework, confidence = _infer_language_and_framework(raw_log)
    root_cause_message = ""

    if parsed["error_signature"]:
        signature_parts = parsed["error_signature"].split(" | ", 2)
        if len(signature_parts) == 3:
            root_cause_message = signature_parts[2]

    redacted_lines = [_redact_text(line) for line in parsed["stack_trace_lines"]][:20]
    redacted_signature = _redact_text(parsed["error_signature"])[:320]
    redacted_root = _redact_text(root_cause_message)[:200]
    fallback_reason = "" if language != "unknown" else "low-signal or mixed-format log"

    return ParsedLogResponse(
        error_type=parsed["error_type"],
        failing_test=parsed["failing_test"],
        stack_trace_lines=redacted_lines,
        error_signature=redacted_signature,
        language=language,
        framework=framework,
        confidence=confidence,
        root_cause_message=redacted_root,
        parser_version="1.1.0",
        fallback_reason=fallback_reason,
    )

app = FastAPI(
    title="AI Engine",
    description="AI/Data Processing service for the Self-Healing CI/CD Platform",
    version="0.1.0",
)


# ---------------------------------------------------------------------------
# Job endpoint — called by the Go orchestrator with a Job-Type header
# ---------------------------------------------------------------------------

VALID_JOB_TYPES = frozenset({"open", "close", "edit", "sync", "logs"})


class _PullRequestPayload(BaseModel):
    number: int = 0
    action: str = ""
    branch: str = ""
    title: str = ""
    body: str = ""
    headsha: str = ""
    basesha: str = ""
    merged: bool = False


class JobRequest(BaseModel):
    """Mirrors the Go AIEngineRequest struct (no json tags → capitalised keys)."""

    Wfid: int
    RepoUrl: str = ""
    PullRequest: _PullRequestPayload = Field(default_factory=_PullRequestPayload)
    Stdout: str = ""
    Stderr: str = ""
    StartTime: str = ""
    EndTime: str = ""
    Errors: str = ""
    Status: str = ""
    OOMKilled: bool = False
    ExitCode: int = 0


async def _process_job(job_type: str, payload: JobRequest) -> None:
    """Background task: analyse logs and POST the result back to the orchestrator."""
    from app.services import orchestrator_client
    from app.services.test_gen import generate_tests

    pr = payload.PullRequest
    pr_dict = {
        "number": pr.number,
        "action": pr.action,
        "branch": pr.branch,
        "title": pr.title,
        "body": pr.body,
        "headsha": pr.headsha,
        "basesha": pr.basesha,
        "merged": pr.merged,
    }

    if job_type == "close":
        await orchestrator_client.send_response(
            payload.Wfid,
            pr_dict,
            done=True,
            summary=f"Workflow {payload.Wfid} closed for PR #{pr.number}.",
        )
        return

    # job_type == "logs"
    if payload.ExitCode == 0:
        await orchestrator_client.send_response(
            payload.Wfid,
            pr_dict,
            done=True,
            summary=f"All tests passed on PR #{pr.number} (exit 0).",
        )
        return

    combined_log = "\n".join(filter(None, [payload.Stdout, payload.Stderr, payload.Errors]))
    package = generate_tests(combined_log, source="ci")
    await orchestrator_client.send_response(
        payload.Wfid,
        pr_dict,
        done=False,
        test_name=f"test_{package['fixture_slug']}.py",
        tests=package["test_stub"].encode(),
    )


@app.post("/")
async def job_handler(request: Request, background_tasks: BackgroundTasks, payload: JobRequest) -> dict:
    """Receive a job from the orchestrator, acknowledge immediately, process in background."""
    job_type = request.headers.get("Job-Type", "")
    if job_type not in VALID_JOB_TYPES:
        raise HTTPException(status_code=400, detail=f"Unknown Job-Type: {job_type!r}")

    if job_type in ("open", "edit", "sync"):
        return {"status": "ok"}

    background_tasks.add_task(_process_job, job_type, payload)
    return {"status": "accepted"}


@app.get("/health")
async def health_check():
    """Liveness probe — returns 200 when the service is running."""
    return {"status": "ok"}


@app.post("/analyze", status_code=202, response_model=AnalyzeAcceptedResponse)
async def analyze(payload: AnalyzeRequest) -> AnalyzeAcceptedResponse:
    """Parse logs into a structured response and persist result by job id."""
    result = _to_user_response(payload.raw_log)
    job_id = str(uuid.uuid4())
    _JOB_RESULTS[job_id] = AnalyzeResultResponse(
        job_id=job_id,
        status="completed",
        result=result,
    )
    return AnalyzeAcceptedResponse(job_id=job_id, status="accepted")


@app.get("/result/{job_id}", response_model=AnalyzeResultResponse)
async def get_result(job_id: str) -> AnalyzeResultResponse:
    """Return the completed parse result for an existing analysis job."""
    job = _JOB_RESULTS.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job_id not found")
    return job


@app.post("/generate-tests", response_model=GeneratedTestsResponse)
async def generate_tests_endpoint(payload: GenerateTestsRequest) -> GeneratedTestsResponse:
    """Analyze a failure log and return a fixture package + pytest stub the user can run immediately."""
    from app.services.test_gen import generate_tests

    package = generate_tests(
        payload.raw_log,
        source=payload.source,
        fixture_name=payload.fixture_name,
    )
    return GeneratedTestsResponse(
        fixture_slug=package["fixture_slug"],
        expected_json=package["expected_json"],
        metadata_json=package["metadata_json"],
        test_stub=package["test_stub"],
        documentation=package["documentation"],
        saved_to_disk=package["saved_to_disk"],
        fixture_path=package["fixture_path"],
    )
