from __future__ import annotations

import re
from typing import Literal

from app.services.log_analyzer import parse_log


_ABS_PATH_RE = re.compile(r"(?:[A-Za-z]:)?/(?:[^\s/:]+/)+[^\s:]+")
_UUID_RE = re.compile(
    r"\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}\b"
)
_LONG_HEX_RE = re.compile(r"\b[0-9a-fA-F]{12,}\b")
_TIME_RE = re.compile(r"\b\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b")
_SECRET_RE = re.compile(r"(?i)\b(?:api[_-]?key|token|password|secret)\s*[:=]\s*\S+")


def _redact_text(value: str) -> str:
    value = _ABS_PATH_RE.sub("<path>", value)
    value = _UUID_RE.sub("<uuid>", value)
    value = _LONG_HEX_RE.sub("<hash>", value)
    value = _TIME_RE.sub("<time>", value)
    return _SECRET_RE.sub("<secret>", value)


def _infer_language_and_framework(raw_log: str) -> tuple[str, str, float]:
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


def create_log_response(raw_log: str) -> dict[str, str | float | list[str]]:
    """Parse and sanitize a log into the response stored by Celery."""
    parsed = parse_log(raw_log)
    language, framework, confidence = _infer_language_and_framework(raw_log)
    root_cause_message = ""

    if parsed["error_signature"]:
        signature_parts = parsed["error_signature"].split(" | ", 2)
        if len(signature_parts) == 3:
            root_cause_message = signature_parts[2]

    return {
        "error_type": parsed["error_type"],
        "failing_test": parsed["failing_test"],
        "stack_trace_lines": [_redact_text(line) for line in parsed["stack_trace_lines"]][:20],
        "error_signature": _redact_text(parsed["error_signature"])[:320],
        "language": language,
        "framework": framework,
        "confidence": confidence,
        "root_cause_message": _redact_text(root_cause_message)[:200],
        "parser_version": "1.1.0",
        "fallback_reason": "" if language != "unknown" else "low-signal or mixed-format log",
    }