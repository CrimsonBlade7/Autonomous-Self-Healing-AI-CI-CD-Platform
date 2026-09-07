from types import SimpleNamespace

import pytest
from fastapi.testclient import TestClient

from app.main import app
from app.services.log_response import create_log_response


client = TestClient(app)


@pytest.fixture(autouse=True)
def celery_task(monkeypatch):
    job_id = "550e8400-e29b-41d4-a716-446655440000"
    tasks_by_job_id = {}

    def fake_delay(raw_log):
        task = SimpleNamespace(
            id=job_id,
            state="SUCCESS",
            ready=lambda: True,
            result=create_log_response(raw_log),
        )
        tasks_by_job_id[job_id] = task
        return task

    monkeypatch.setattr("app.main.analyze_log.delay", fake_delay)
    monkeypatch.setattr("app.main.celery_app.AsyncResult", lambda requested_job_id: tasks_by_job_id[requested_job_id])
    return tasks_by_job_id


def test_analyze_accepts_log_and_returns_job_id():
    payload = {
        "raw_log": "AssertionError: boom",
        "source": "user",
    }

    response = client.post("/analyze", json=payload)

    assert response.status_code == 202
    body = response.json()
    assert body["status"] == "accepted"
    assert isinstance(body["job_id"], str)
    assert len(body["job_id"]) > 10


def test_result_endpoint_returns_user_facing_schema():
    payload = {
        "raw_log": "Traceback (most recent call last):\nAssertionError: divide by zero\n",
        "source": "ci",
    }

    accepted = client.post("/analyze", json=payload)
    job_id = accepted.json()["job_id"]

    result_response = client.get(f"/result/{job_id}")
    assert result_response.status_code == 200

    body = result_response.json()
    assert body["status"] == "completed"
    assert body["job_id"] == job_id

    result = body["result"]
    required_keys = {
        "error_type",
        "failing_test",
        "stack_trace_lines",
        "error_signature",
        "language",
        "framework",
        "confidence",
        "root_cause_message",
        "parser_version",
        "fallback_reason",
    }
    assert required_keys.issubset(set(result.keys()))


def test_result_endpoint_redacts_sensitive_tokens():
    payload = {
        "raw_log": (
            "TypeError: secret=abc123 at /Users/demo/project/file.py "
            "request_id=550e8400-e29b-41d4-a716-446655440000"
        ),
        "source": "user",
    }

    accepted = client.post("/analyze", json=payload)
    job_id = accepted.json()["job_id"]

    result = client.get(f"/result/{job_id}").json()["result"]
    signature = result["error_signature"]

    assert "<secret>" in signature
    assert "<path>" in signature
    assert "<uuid>" in signature
