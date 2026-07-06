"""Rust log parser for cargo test and compiler failures."""

from __future__ import annotations

from .base import ParsedError
from .fallback_parser import FallbackLogParser


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
        return FallbackLogParser().parse(raw_log)