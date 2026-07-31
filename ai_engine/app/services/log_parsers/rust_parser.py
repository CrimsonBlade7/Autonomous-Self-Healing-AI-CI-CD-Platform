"""Rust log parser for cargo test and compiler failures."""

from __future__ import annotations

import re

from .base import ParsedError, build_error, empty_error, normalize_lines


_TEST_FAIL_RE = re.compile(r"^test\s+(?P<name>\S+)\s+\.\.\.\s+FAILED$")
_PANIC_LINE_RE = re.compile(r"panicked at\s+(?P<msg>.+)$", re.IGNORECASE)
_COMPILER_ERR_RE = re.compile(r"^error\[(?P<code>E\d+)\]:\s*(?P<msg>.+)$", re.IGNORECASE)


class RustLogParser:
    """Detect and parse Rust CI logs."""

    name = "rust"

    def can_parse(self, raw_log: str) -> bool:
        lowered = raw_log.lower()
        return any(
            marker in lowered
            for marker in (
                "thread '",
                "panicked at",
                "error[e",
                "cargo test",
                "could not compile",
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
            match = _TEST_FAIL_RE.match(line)
            if match:
                return match.group("name")
        return "unknown"

    def _extract_context_block(self, lines: list[str]) -> list[str]:
        start_idx = -1
        for i, line in enumerate(lines):
            lowered = line.lower()
            if "panicked at" in lowered or lowered.startswith("error["):
                start_idx = i
                break

        if start_idx == -1:
            return [
                line
                for line in lines
                if any(
                    token in line.lower()
                    for token in ("failed", "panic", "error", "could not compile")
                )
            ][:20]

        return lines[start_idx : start_idx + 20]

    def _extract_error(
        self,
        lines: list[str],
        stack_trace_lines: list[str],
    ) -> tuple[str, str]:
        search_lines = stack_trace_lines if stack_trace_lines else lines

        for i, line in enumerate(search_lines):
            panic_match = _PANIC_LINE_RE.search(line)
            if panic_match:
                message = panic_match.group("msg").strip()
                if i + 1 < len(search_lines):
                    next_line = search_lines[i + 1].strip()
                    if next_line and not next_line.lower().startswith("note:"):
                        message = f"{message} {next_line}".strip()
                return "PanicError", message

            compiler_match = _COMPILER_ERR_RE.match(line)
            if compiler_match:
                return "RustCompileError", compiler_match.group("msg").strip()

        return "UnknownError", ""