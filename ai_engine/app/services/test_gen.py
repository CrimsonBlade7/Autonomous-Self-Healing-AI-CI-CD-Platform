"""Generate test fixtures and test stubs from raw CI failure logs.

Given a raw log this module:
1. Parses it into a structured error via log_analyzer.
2. Builds a fixture package (input.log, expected.json, metadata.json).
3. Generates a pytest test stub targeting that fixture.
4. Returns everything inline so the caller can persist or forward to the user.
"""

from __future__ import annotations

import json
import re
import textwrap
from pathlib import Path
from typing import TypedDict

from app.services.log_analyzer import ParsedError, parse_log

# Resolved at import time so the path is correct regardless of cwd.
_FIXTURES_ROOT = Path(__file__).resolve().parents[2] / "tests" / "services" / "fixtures"


# ---------------------------------------------------------------------------
# Internal helpers (duplicated from create_log_fixture.py intentionally —
# the CLI tool and this runtime service must not share mutable state).
# ---------------------------------------------------------------------------

def _slugify(name: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", name.lower()).strip("_")


def _infer_language_and_framework(raw_log: str) -> tuple[str, str, float]:
    lowered = raw_log.lower()
    if any(t in lowered for t in ("traceback", "pytest", "assertionerror", "modulenotfounderror")):
        return "python", "pytest", 0.88
    if any(t in lowered for t in ("jest", "vitest", "npm err!", "typeerror:")):
        return "node", "jest", 0.78
    if any(t in lowered for t in ("--- fail:", "panic:", "go test")):
        return "go", "go test", 0.82
    if any(t in lowered for t in ("caused by:", "exception in thread", "mvn test", "gradle")):
        return "java", "maven", 0.80
    if any(t in lowered for t in ("cargo test", "panicked at", "error[e")):
        return "rust", "cargo", 0.81
    return "unknown", "unknown", 0.35


def _confidence_band(score: float) -> str:
    if score >= 0.80:
        return "high"
    if score >= 0.60:
        return "medium"
    return "low"


# ---------------------------------------------------------------------------
# Test-stub generation
# ---------------------------------------------------------------------------

def _build_test_stub(parsed: ParsedError, fixture_slug: str) -> str:
    """Return a pytest file that exercises the fixture against the live parser."""
    assertions: list[str] = []
    if parsed["error_type"] and parsed["error_type"] != "UnknownError":
        assertions.append(f'    assert result["error_type"] == {json.dumps(parsed["error_type"])}')
    if parsed["failing_test"] and parsed["failing_test"] != "unknown":
        assertions.append(f'    assert result["failing_test"] == {json.dumps(parsed["failing_test"])}')
    if parsed["error_signature"]:
        assertions.append(f'    assert result["error_signature"] == {json.dumps(parsed["error_signature"])}')
    if parsed["stack_trace_lines"]:
        assertions.append(f'    assert len(result["stack_trace_lines"]) == {len(parsed["stack_trace_lines"])}')
    if not assertions:
        assertions.append('    assert result["error_type"] != ""  # parser must produce some output')

    assertion_block = "\n".join(assertions)

    return textwrap.dedent(f"""\
        from pathlib import Path
        import json
        import pytest
        from app.services.log_analyzer import parse_log

        _FIXTURE = Path(__file__).parent / "fixtures" / "{fixture_slug}"


        def test_{fixture_slug}_golden_output():
            raw = (_FIXTURE / "input.log").read_text()
            expected = json.loads((_FIXTURE / "expected.json").read_text())
            assert parse_log(raw) == expected


        def test_{fixture_slug}_determinism():
            raw = (_FIXTURE / "input.log").read_text()
            assert parse_log(raw) == parse_log(raw)


        def test_{fixture_slug}_schema_keys():
            raw = (_FIXTURE / "input.log").read_text()
            result = parse_log(raw)
{assertion_block}
    """)


def _build_documentation(
    parsed: ParsedError,
    metadata: dict,
    fixture_slug: str,
    test_stub: str,
) -> str:
    trace_block = "\n".join(parsed["stack_trace_lines"]) or "(none captured)"
    return textwrap.dedent(f"""\
        # Generated Test: `{fixture_slug}`

        ## Failure Summary
        - **Error type**: `{parsed["error_type"]}`
        - **Failing test**: `{parsed["failing_test"]}`
        - **Error signature**: `{parsed["error_signature"]}`
        - **Language / framework**: {metadata["language"]} / {metadata["framework"]}
        - **Confidence**: {metadata["expected_confidence_band"]}

        ## Stack Trace
        ```
        {trace_block}
        ```

        ## Fixture Location
        `tests/services/fixtures/{fixture_slug}/`
        - `input.log` — raw log input
        - `expected.json` — golden parser output to validate against
        - `metadata.json` — source, language, framework, confidence band

        ## Generated Test Stub
        Save this as `tests/services/test_{fixture_slug}.py` and run `pytest`.

        ```python
        {test_stub}
        ```

        ## Next Steps
        1. Review `expected.json` and correct any parser misdetections.
        2. Add the test stub to `tests/services/` and confirm `pytest` passes.
        3. Update `notes` in `metadata.json` with root-cause context.
    """)


# ---------------------------------------------------------------------------
# Public interface
# ---------------------------------------------------------------------------

class GeneratedTestPackage(TypedDict):
    fixture_slug: str
    input_log: str
    expected_json: dict
    metadata_json: dict
    test_stub: str
    documentation: str
    saved_to_disk: bool
    fixture_path: str


def generate_tests(
    raw_log: str,
    source: str = "ci",
    fixture_name: str | None = None,
) -> GeneratedTestPackage:
    """Parse *raw_log* and produce a complete, ready-to-run test fixture package.

    Args:
        raw_log:      Raw CI or user-provided failure log.
        source:       ``"ci"`` or ``"user"`` — recorded in metadata.json.
        fixture_name: Optional name override; auto-derived from failing test otherwise.

    Returns:
        A :class:`GeneratedTestPackage` with fixture content, a pytest stub,
        and human-readable documentation.  Files are also written under
        ``tests/services/fixtures/<slug>/`` when the directory does not yet exist.
    """
    parsed = parse_log(raw_log)
    language, framework, confidence = _infer_language_and_framework(raw_log)
    band = _confidence_band(confidence)

    base_name = fixture_name or parsed["failing_test"] or parsed["error_type"] or "unknown_failure"
    fixture_slug = _slugify(base_name)

    expected: dict = {
        "error_type": parsed["error_type"],
        "failing_test": parsed["failing_test"],
        "stack_trace_lines": parsed["stack_trace_lines"],
        "error_signature": parsed["error_signature"],
    }
    metadata: dict = {
        "name": fixture_slug,
        "source": source,
        "language": language,
        "framework": framework,
        "expected_confidence_band": band,
        "notes": "Auto-generated by test_gen service from CI failure log.",
    }

    test_stub = _build_test_stub(parsed, fixture_slug)
    documentation = _build_documentation(parsed, metadata, fixture_slug, test_stub)

    fixture_dir = _FIXTURES_ROOT / fixture_slug
    saved = False
    try:
        fixture_dir.mkdir(parents=True, exist_ok=False)
        (fixture_dir / "input.log").write_text(raw_log, encoding="utf-8")
        (fixture_dir / "expected.json").write_text(json.dumps(expected, indent=2) + "\n", encoding="utf-8")
        (fixture_dir / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
        saved = True
    except FileExistsError:
        pass  # fixture already exists; return content without overwriting

    return GeneratedTestPackage(
        fixture_slug=fixture_slug,
        input_log=raw_log,
        expected_json=expected,
        metadata_json=metadata,
        test_stub=test_stub,
        documentation=documentation,
        saved_to_disk=saved,
        fixture_path=str(fixture_dir),
    )
