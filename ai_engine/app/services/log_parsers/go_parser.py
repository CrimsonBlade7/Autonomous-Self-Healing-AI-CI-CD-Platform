"""Go log parser for test and build failures."""

from __future__ import annotations

import re

from .base import ParsedError, build_error, empty_error, normalize_lines


_GO_TEST_FAIL_RE = re.compile(r"^--- FAIL: (?P<name>\S+)")
_GO_COMPILE_ERR_RE = re.compile(
    r"^[\w./-]+\.go:\d+:\d+:\s*(?P<msg>.+)$"
)


class GoLogParser:
    """Detect and parse Go CI logs."""

    name = "go"

    def can_parse(self, raw_log: str) -> bool:
        lowered = raw_log.lower()
        return any(
            marker in lowered
            for marker in (
                "panic:",
                "--- fail:",
                "go test",
                "cannot use",
                "undefined:",
                "build failed",
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
            match = _GO_TEST_FAIL_RE.match(line)
            if match:
                return match.group("name")
        return "unknown"

    def _extract_context_block(self, lines: list[str]) -> list[str]:
        start_idx = -1
        for i, line in enumerate(lines):
            lowered = line.lower()
            if lowered.startswith("panic:") or lowered.startswith("--- fail:"):
                start_idx = i
                break
            if _GO_COMPILE_ERR_RE.match(line):
                start_idx = i
                break

        if start_idx == -1:
            return [
                line
                for line in lines
                if any(
                    token in line.lower()
                    for token in ("panic", "fail", "undefined:", "cannot use")
                )
            ][:20]

        return lines[start_idx : start_idx + 20]

    def _extract_error(
        self,
        lines: list[str],
        stack_trace_lines: list[str],
    ) -> tuple[str, str]:
        search_lines = stack_trace_lines if stack_trace_lines else lines

        for line in search_lines:
            lowered = line.lower()
            if lowered.startswith("panic:"):
                return "GoPanicError", line.split("panic:", 1)[1].strip()

            compile_match = _GO_COMPILE_ERR_RE.match(line)
            if compile_match:
                return "GoCompileError", compile_match.group("msg").strip()

        return "UnknownError", ""