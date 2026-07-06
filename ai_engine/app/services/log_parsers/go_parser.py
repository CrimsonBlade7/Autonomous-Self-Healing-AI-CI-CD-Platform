"""Go log parser for test and build failures."""

from __future__ import annotations

from .base import ParsedError
from .fallback_parser import FallbackLogParser


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
        return FallbackLogParser().parse(raw_log)