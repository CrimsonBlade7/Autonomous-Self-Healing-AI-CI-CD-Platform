"""Node.js / TypeScript log parser."""

from __future__ import annotations

import re

from .base import ParsedError, build_error, empty_error, normalize_lines


_JEST_FAIL_RE = re.compile(r"^\s*●\s+(?P<name>.+)$")
_ERROR_LINE_RE = re.compile(
    r"^(?P<etype>(?:Type|Reference|Syntax|Range)Error):\s*(?P<msg>.+)$"
)
_TS_ERROR_RE = re.compile(r"error\s+TS\d+:\s*(?P<msg>.+)", re.IGNORECASE)


class NodeLogParser:
    """Detect and parse Node.js and TypeScript CI logs."""

    name = "node"

    def can_parse(self, raw_log: str) -> bool:
        lowered = raw_log.lower()
        return any(
            marker in lowered
            for marker in (
                "npm err!",
                "yarn error",
                "pnpm error",
                "jest",
                "vitest",
                "typescript",
                "typeerror:",
                "referenceerror:",
                "syntaxerror:",
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
            match = _JEST_FAIL_RE.match(line)
            if match:
                return match.group("name").strip()

        for line in lines:
            if line.startswith("FAIL "):
                return line.replace("FAIL", "", 1).strip()

        return "unknown"

    def _extract_context_block(self, lines: list[str]) -> list[str]:
        start_idx = -1
        for i, line in enumerate(lines):
            lowered = line.lower()
            if any(
                token in lowered
                for token in (
                    "typeerror:",
                    "referenceerror:",
                    "syntaxerror:",
                    "error ts",
                    "npm err!",
                )
            ):
                start_idx = i
                break

        if start_idx == -1:
            return [
                line
                for line in lines
                if any(
                    token in line.lower()
                    for token in ("error", "fail", "exception", "cannot")
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
            err_match = _ERROR_LINE_RE.match(line)
            if err_match:
                return err_match.group("etype"), err_match.group("msg").strip()

            ts_match = _TS_ERROR_RE.search(line)
            if ts_match:
                return "TypeScriptError", ts_match.group("msg").strip()

        return "UnknownError", ""