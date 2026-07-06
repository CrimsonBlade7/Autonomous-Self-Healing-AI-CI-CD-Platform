"""Node.js / TypeScript log parser."""

from __future__ import annotations

from .base import ParsedError
from .fallback_parser import FallbackLogParser


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
        return FallbackLogParser().parse(raw_log)