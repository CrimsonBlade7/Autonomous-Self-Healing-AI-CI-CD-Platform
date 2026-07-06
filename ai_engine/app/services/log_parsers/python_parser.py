"""Python log parser for pytest, unittest, and traceback output."""

from __future__ import annotations

from .base import (
    ParsedError,
    build_error,
    empty_error,
    extract_error_type_and_message,
    extract_failing_test,
    find_traceback_block,
    normalize_lines,
)


class PythonLogParser:
    """Detect and parse Python CI logs."""

    name = "python"

    def can_parse(self, raw_log: str) -> bool:
        lowered = raw_log.lower()
        return any(
            marker in lowered
            for marker in (
                "traceback (most recent call last):",
                "pytest",
                "unittest",
                "assertionerror",
                "module not founderror",
                "modulenotfounderror",
                "importerror",
            )
        )

    def parse(self, raw_log: str) -> ParsedError:
        if not raw_log.strip():
            return empty_error()

        lines = normalize_lines(raw_log)
        if not lines:
            return empty_error()

        failing_test = extract_failing_test(lines)
        stack_trace_lines = find_traceback_block(lines)
        error_type, message = extract_error_type_and_message(
            stack_trace_lines if stack_trace_lines else lines
        )
        return build_error(error_type, failing_test, stack_trace_lines, message)