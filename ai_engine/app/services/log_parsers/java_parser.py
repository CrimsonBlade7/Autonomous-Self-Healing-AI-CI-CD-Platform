"""Java log parser for Maven, Gradle, and stack trace failures."""

from __future__ import annotations

from .base import ParsedError
from .fallback_parser import FallbackLogParser


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
        return FallbackLogParser().parse(raw_log)