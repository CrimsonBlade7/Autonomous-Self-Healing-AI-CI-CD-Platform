"""Parser registry for raw CI logs."""

from __future__ import annotations

from .base import ParsedError
from .fallback_parser import FallbackLogParser
from .go_parser import GoLogParser
from .java_parser import JavaLogParser
from .node_parser import NodeLogParser
from .python_parser import PythonLogParser
from .rust_parser import RustLogParser


_PARSERS = (
    PythonLogParser(),
    NodeLogParser(),
    GoLogParser(),
    JavaLogParser(),
    RustLogParser(),
    FallbackLogParser(),
)


def parse_log(raw_log: str) -> ParsedError:
    """Parse a raw CI log into a compact failure object."""
    for parser in _PARSERS:
        if parser.can_parse(raw_log):
            return parser.parse(raw_log)

    return FallbackLogParser().parse(raw_log)