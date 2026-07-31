"""Java log parser for Maven, Gradle, and stack trace failures."""

from __future__ import annotations

import re

from .base import ParsedError, build_error, empty_error, normalize_lines


_MAVEN_TEST_CLASS_RE = re.compile(r"-\s+in\s+(?P<name>[\w.$]+)$")
_CAUSED_BY_RE = re.compile(r"^Caused by:\s+(?P<etype>[\w.$]+)(?::\s*(?P<msg>.*))?$")
_EXCEPTION_THREAD_RE = re.compile(
    r'^Exception in thread "[^"]+"\s+(?P<etype>[\w.$]+)(?::\s*(?P<msg>.*))?$'
)


class JavaLogParser:
    """Detect and parse Java CI logs."""

    name = "java"

    def can_parse(self, raw_log: str) -> bool:
        lowered = raw_log.lower()
        return any(
            marker in lowered
            for marker in (
                "exception in thread",
                "caused by:",
                "[error]",
                "mvn test",
                "gradle",
                "failed to execute goal",
            )
        )

    def parse(self, raw_log: str) -> ParsedError:
        if not raw_log.strip():
            return empty_error()

        lines = normalize_lines(raw_log)
        if not lines:
            return empty_error()

        failing_test = self._extract_failing_test(lines)
        stack_trace_lines = self._extract_context_block(lines)
        error_type, message = self._extract_error(lines, stack_trace_lines)
        return build_error(error_type, failing_test, stack_trace_lines, message)

    def _extract_failing_test(self, lines: list[str]) -> str:
        for line in lines:
            match = _MAVEN_TEST_CLASS_RE.search(line)
            if match:
                return match.group("name")
        return "unknown"

    def _extract_context_block(self, lines: list[str]) -> list[str]:
        start_idx = -1
        for i, line in enumerate(lines):
            lowered = line.lower()
            if lowered.startswith("exception in thread") or lowered.startswith("caused by:"):
                start_idx = i
                break
            if lowered.startswith("[error]") and "exception" in lowered:
                start_idx = i
                break

        if start_idx == -1:
            return [
                line
                for line in lines
                if any(
                    token in line.lower()
                    for token in ("exception", "error", "failed", "caused by")
                )
            ][:20]

        return lines[start_idx : start_idx + 20]

    def _extract_error(
        self,
        lines: list[str],
        stack_trace_lines: list[str],
    ) -> tuple[str, str]:
        search_lines = stack_trace_lines if stack_trace_lines else lines

        for line in reversed(search_lines):
            caused_by_match = _CAUSED_BY_RE.match(line)
            if caused_by_match:
                return (
                    caused_by_match.group("etype").split(".")[-1],
                    (caused_by_match.group("msg") or "").strip(),
                )

        for line in search_lines:
            thread_match = _EXCEPTION_THREAD_RE.match(line)
            if thread_match:
                return (
                    thread_match.group("etype").split(".")[-1],
                    (thread_match.group("msg") or "").strip(),
                )

        return "UnknownError", ""