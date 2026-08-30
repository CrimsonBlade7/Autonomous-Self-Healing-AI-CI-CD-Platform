# User-Facing Log Response Specification

## Scope
Defines what the AI engine returns to users after parsing CI or user-provided test logs.

## Response Object (v1 Required)
- `error_type`: Canonical failure type.
- `failing_test`: Best-effort test identifier, else `unknown`.
- `stack_trace_lines`: Compact list of the most relevant failure lines.
- `error_signature`: Normalized compact summary for retrieval and deduplication.

## Recommended Additive Fields (v1.1)
- `language`: `python | node | go | java | rust | unknown`
- `framework`: `pytest | jest | go test | maven | gradle | cargo | unknown`
- `confidence`: float between 0.0 and 1.0
- `root_cause_message`: brief root-cause string
- `parser_version`: parser semantic version
- `fallback_reason`: non-empty only when fallback parser is used

## Redaction Policy
Before returning any user-visible payload:
1. Mask absolute file paths to `<path>`.
2. Mask UUIDs to `<uuid>`.
3. Mask long hex strings and hashes to `<hash>`.
4. Mask timestamps to `<time>`.
5. Mask obvious secrets and tokens to `<secret>`.

## Truncation Policy
1. Limit `stack_trace_lines` to a fixed maximum (recommended: 20).
2. Limit `error_signature` message portion to fixed length (recommended: 200 chars).
3. Preserve the most semantically important lines around the root cause.

## Confidence Interpretation
- High (>=0.80): ecosystem and root cause highly likely correct.
- Medium (0.50-0.79): likely correct but may need human review.
- Low (<0.50): fallback/ambiguous parse; user should inspect full logs.

## User Messaging Rules
1. Keep output deterministic and concise.
2. Do not expose raw secrets or environment internals.
3. Include `fallback_reason` when confidence is low.
4. If parsing fails, return safe defaults rather than raising errors.

## Example Response
{
  "error_type": "AssertionError",
  "failing_test": "tests/test_math.py::test_divide",
  "stack_trace_lines": [
    "Traceback (most recent call last):",
    "AssertionError: divide by zero guard missing"
  ],
  "error_signature": "python:assertionerror | tests/test_math.py::test_divide | divide by zero guard missing",
  "language": "python",
  "framework": "pytest",
  "confidence": 0.88,
  "root_cause_message": "divide by zero guard missing",
  "parser_version": "1.1.0",
  "fallback_reason": ""
}

## Integration Note
The same schema applies whether logs originate from CI pipeline jobs or from user-supplied test runs.
