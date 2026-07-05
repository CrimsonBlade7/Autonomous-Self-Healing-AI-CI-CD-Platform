"""Log parsing for CI job failures.

This module turns raw test/build logs into a fixed schema that later pipeline
steps can consume deterministically.
"""

from __future__ import annotations

import re
from typing import TypedDict


class ParsedError(TypedDict):
	"""Structured representation of a failure extracted from raw logs."""

	error_type: str
	failing_test: str
	stack_trace_lines: list[str]
	error_signature: str


_ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*m")
_PYTEST_NODE_RE = re.compile(r"(?P<node>[\w./\\-]+::[\w\[\]\\-]+)")
_UNITTEST_FAIL_RE = re.compile(r"^(?:FAIL|ERROR):\s+(?P<test>\S+)")
_EXCEPTION_LINE_RE = re.compile(
	r"^(?P<etype>[A-Za-z_][\w.]*(?:Error|Exception))(?::\s*(?P<msg>.*))?$"
)


def _normalize_lines(raw_log: str) -> list[str]:
	"""Normalize and clean raw log text into non-empty lines."""
	lines: list[str] = []
	for line in raw_log.splitlines():
		clean = _ANSI_ESCAPE_RE.sub("", line).strip()
		if clean:
			lines.append(clean)
	return lines


def _extract_failing_test(lines: list[str]) -> str:
	"""Extract a likely failing test identifier from pytest/unittest output."""
	for line in lines:
		pytest_match = _PYTEST_NODE_RE.search(line)
		if pytest_match:
			return pytest_match.group("node")

		unittest_match = _UNITTEST_FAIL_RE.match(line)
		if unittest_match:
			return unittest_match.group("test")

	return "unknown"


def _find_traceback_block(lines: list[str]) -> list[str]:
	"""Return traceback section if present; otherwise fallback error-like lines."""
	start_idx = -1
	for i, line in enumerate(lines):
		if line.startswith("Traceback (most recent call last):"):
			start_idx = i
			break

	if start_idx == -1:
		# Fallback: keep lines that likely carry error context.
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


def _extract_error_type_and_message(lines: list[str]) -> tuple[str, str]:
	"""Extract exception class and human-readable message from log lines."""
	for line in reversed(lines):
		match = _EXCEPTION_LINE_RE.match(line)
		if match:
			error_type = match.group("etype")
			message = (match.group("msg") or "").strip()
			return error_type, message

	# Fallback for common assertion-only failures.
	for line in reversed(lines):
		if "assertionerror" in line.lower() or line.lower().startswith("assert "):
			return "AssertionError", line

	return "UnknownError", ""


def _build_signature(error_type: str, failing_test: str, message: str) -> str:
	"""Build compact signature text used by the embedding/retrieval pipeline."""
	message_part = message if message else "no message"
	return f"{error_type} | {failing_test} | {message_part}"


def parse_log(raw_log: str) -> ParsedError:
	"""Parse raw CI logs into structured failure metadata.

	The function is intentionally deterministic: same input gives same output.
	"""
	if not raw_log.strip():
		return {
			"error_type": "UnknownError",
			"failing_test": "unknown",
			"stack_trace_lines": [],
			"error_signature": "UnknownError | unknown | empty log",
		}

	lines = _normalize_lines(raw_log)
	failing_test = _extract_failing_test(lines)
	stack_trace_lines = _find_traceback_block(lines)
	error_type, message = _extract_error_type_and_message(
		stack_trace_lines if stack_trace_lines else lines
	)
	error_signature = _build_signature(error_type, failing_test, message)

	return {
		"error_type": error_type,
		"failing_test": failing_test,
		"stack_trace_lines": stack_trace_lines,
		"error_signature": error_signature,
	}


def analyze_log(raw_log: str) -> ParsedError:
	"""Alias kept for readability in service-layer call sites."""
	return parse_log(raw_log)
