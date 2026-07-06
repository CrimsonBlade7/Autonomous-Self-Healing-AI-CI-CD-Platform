"""Shared helpers and schema for log parser implementations."""

from __future__ import annotations

import re
from typing import Protocol, TypedDict


class ParsedError(TypedDict):
    """Structured representation of a failure extracted from raw logs."""

    error_type: str
    failing_test: str
    stack_trace_lines: list[str]
    error_signature: str


class LogParser(Protocol):
    """Contract for language-specific log parsers."""

    name: str

    def can_parse(self, raw_log: str) -> bool:
        """Return True when the parser should handle the log."""

    def parse(self, raw_log: str) -> ParsedError:
        """Parse the raw log into the shared failure schema."""


_ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*m")
_PYTEST_NODE_RE = re.compile(r"(?P<node>[\w./\\-]+::[\w\[\]\\-]+)")
_UNITTEST_FAIL_RE = re.compile(r"^(?:FAIL|ERROR):\s+(?P<test>\S+)")
_EXCEPTION_LINE_RE = re.compile(
    r"^(?P<etype>[A-Za-z_][\w.]*(?:Error|Exception))(?::\s*(?P<msg>.*))?$"
)


def normalize_lines(raw_log: str) -> list[str]:
    """Normalize and clean raw log text into non-empty lines."""
    lines: list[str] = []
    for line in raw_log.splitlines():
        clean = _ANSI_ESCAPE_RE.sub("", line).strip()
        if clean:
            lines.append(clean)
    return lines


def extract_failing_test(lines: list[str]) -> str:
    """Extract a likely failing test identifier from pytest/unittest output."""
    for line in lines:
        pytest_match = _PYTEST_NODE_RE.search(line)
        if pytest_match:
            return pytest_match.group("node")

        unittest_match = _UNITTEST_FAIL_RE.match(line)
        if unittest_match:
            return unittest_match.group("test")

    return "unknown"


def find_traceback_block(lines: list[str]) -> list[str]:
    """Return traceback section if present; otherwise fallback error-like lines."""
    start_idx = -1
    for i, line in enumerate(lines):
        if line.startswith("Traceback (most recent call last):"):
            start_idx = i
            break

    if start_idx == -1:
        return [
            line
            for line in lines
            if any(
                token in line.lower()
                for token in ("error", "exception", "failed", "assert")
            )
        ][:20]

    block: list[str] = []
    for line in lines[start_idx:]:
        block.append(line)
        if _EXCEPTION_LINE_RE.match(line):
            break
    return block


def extract_error_type_and_message(lines: list[str]) -> tuple[str, str]:
    """Extract exception class and human-readable message from log lines."""
    for line in reversed(lines):
        match = _EXCEPTION_LINE_RE.match(line)
        if match:
            error_type = match.group("etype")
            message = (match.group("msg") or "").strip()
            return error_type, message

    for line in reversed(lines):
        if "assertionerror" in line.lower() or line.lower().startswith("assert "):
            return "AssertionError", line

    return "UnknownError", ""


def build_signature(error_type: str, failing_test: str, message: str) -> str:
    """Build compact signature text used by the embedding/retrieval pipeline."""
    message_part = message if message else "no message"
    return f"{error_type} | {failing_test} | {message_part}"


def empty_error() -> ParsedError:
    """Return the canonical empty/unknown error object."""
    return {
        "error_type": "UnknownError",
        "failing_test": "unknown",
        "stack_trace_lines": [],
        "error_signature": "UnknownError | unknown | empty log",
    }


def build_error(
    error_type: str,
    failing_test: str,
    stack_trace_lines: list[str],
    message: str,
) -> ParsedError:
    """Assemble the shared error schema."""
    return {
        "error_type": error_type,
        "failing_test": failing_test,
        "stack_trace_lines": stack_trace_lines,
        "error_signature": build_signature(error_type, failing_test, message),
    }