# Multi-Language Log Parser: Implementation Strategy

## 1) Goal
Build a deterministic log parser that converts raw CI output into a compact, stable failure object that works across multiple ecosystems (Python, Node/TypeScript, Go, Java, Rust).

Primary output schema (v1):
- `error_type: str`
- `failing_test: str`
- `stack_trace_lines: list[str]`
- `error_signature: str`

Recommended v1.1 extensions:
- `language: str` (python/node/go/java/rust/unknown)
- `framework: str` (pytest/jest/go test/maven/...)
- `confidence: float` (0.0-1.0)
- `root_cause_message: str`

## 2) Non-Goals (for v1)
- Perfect parsing for every test runner on day one.
- ML-based parsing/classification.
- Deep semantic code reasoning in parser layer.

## 3) Design Principles
- Deterministic: same input log => same parsed output.
- Layered: generic pipeline + per-language detectors.
- Fail-safe: never crash; always return valid schema.
- Stable signatures: normalize noisy tokens for better retrieval quality.
- Small, testable units: each detector has fixture-based tests.

## 4) Target Architecture
Implement parser pipeline in this order:
1. Normalize text
2. Detect language and framework
3. Extract failing test identifier
4. Extract root-cause block (traceback/panic/compiler message)
5. Extract canonical `error_type` and `root_cause_message`
6. Build normalized compact `error_signature`
7. Return schema + confidence

Suggested module split under `ai_engine/app/services/`:
- `log_analyzer.py` (public entrypoint)
- `log_parsers/base.py` (shared interfaces/helpers)
- `log_parsers/python_parser.py`
- `log_parsers/node_parser.py`
- `log_parsers/go_parser.py`
- `log_parsers/java_parser.py`
- `log_parsers/rust_parser.py`
- `log_parsers/fallback_parser.py`

## 5) Detector Strategy (Rule-Based)
Use simple scoring rules per detector.

Example signals:
- Python: `Traceback (most recent call last):`, `pytest`, `AssertionError`, `ModuleNotFoundError`
- Node: `npm ERR!`, `TypeError:`, `ReferenceError:`, `Jest`, `Vitest`
- Go: `panic:`, `--- FAIL:`, `go test`, `cannot use ... as ...`
- Java: `Exception in thread`, `Caused by:`, `[ERROR]`, `mvn test`, `gradle test`
- Rust: `thread '...' panicked`, `error[E`, `cargo test`

Select detector with highest score above threshold. If below threshold, use fallback parser.

## 6) Signature Normalization Rules
Normalize high-variance message content before building signature:
- Lowercase
- Collapse whitespace
- Replace UUIDs, long hex values, timestamps with placeholders
- Replace absolute paths with `<path>`
- Optional: replace numeric literals with `<num>` when not semantically meaningful
- Truncate message to fixed max length (for example 200 chars)

Signature template (v1.1):
`{language}:{error_type} | {failing_test} | {normalized_message}`

## 7) Phased Delivery Plan

### Phase 1 (0.5-1 day): Parser Foundation
Deliverables:
- Shared parser interface and fallback parser.
- Preserve current schema compatibility.
- Unit tests for empty/noisy logs and fallback behavior.

Acceptance:
- No regressions on current Python examples.
- Parser always returns valid object.

### Phase 2 (1 day): Python Hardening
Deliverables:
- Improve traceback extraction to capture final/root exception in chained errors.
- Improve pytest/unittest failing-test extraction.
- Add signature normalization.

Acceptance:
- Handles assertion failures, import errors, and chained exceptions.
- Better `error_signature` stability under path/ID noise.

### Phase 3 (1 day): Node + Go Detectors
Deliverables:
- Node and Go detector modules + extraction rules.
- Fixture suite with representative CI logs.

Acceptance:
- Correctly identifies language and major error types for fixtures.
- Confidence score implemented and non-zero for matched detectors.

### Phase 4 (1 day): Java + Rust Detectors
Deliverables:
- Java and Rust detector modules + extraction rules.
- Additional fixtures and regression tests.

Acceptance:
- Correct root-cause extraction for common build/test failures.
- No parser crashes on mixed or noisy logs.

### Phase 5 (0.5 day): Integration + Observability
Deliverables:
- Wire parser metadata into RAG input construction.
- Add lightweight metrics/logging:
  - detector selected
  - confidence
  - unknown/fallback rate

Acceptance:
- RAG receives compact, consistent signatures.
- Unknown/fallback usage measurable.

## 8) Test Plan
Use fixture-driven tests under `ai_engine/tests/services/log_parsers/`.

Fixture categories:
- Python: pytest fail, unittest fail, chained exceptions
- Node: jest assertion failure, runtime TypeError
- Go: panic, compile error, go test failure
- Java: stack trace with `Caused by`, maven wrapper logs
- Rust: panic, compiler diagnostic
- Generic: truncated logs, ANSI-heavy logs, mixed output

Assertions per fixture:
- `error_type` expected
- `failing_test` expected or `unknown`
- root cause line appears in `stack_trace_lines`
- signature normalized and length-bounded
- parser does not throw

## 9) Risk Register and Mitigations
- Risk: false language detection in mixed logs.
  - Mitigation: scoring threshold + fallback parser + confidence.
- Risk: over-normalization removes useful semantics.
  - Mitigation: normalize only known noisy token types; keep raw stack snippet.
- Risk: parser growth becomes hard to maintain.
  - Mitigation: strict detector interface + fixtures + CI regression tests.

## 10) Definition of Done (v1)
- Python/Node/Go/Java/Rust detectors implemented.
- Fallback parser covers unknown formats safely.
- >= 30 fixture tests passing across ecosystems.
- Existing parser interface still works for current callers.
- Compact output quality is acceptable for retrieval and patch generation.

## 11) Suggested Immediate Next 3 Tasks
1. Create detector interface and fallback parser skeleton.
2. Implement Python hardening and message normalization.
3. Add first fixture suite (Python + Node + Go) and CI test job.
