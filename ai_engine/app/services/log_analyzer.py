"""Public log parsing entrypoint for CI job failures."""

from __future__ import annotations

from .log_parsers import parse_log


def analyze_log(raw_log: str):
	"""Alias kept for readability in service-layer call sites."""
	return parse_log(raw_log)
