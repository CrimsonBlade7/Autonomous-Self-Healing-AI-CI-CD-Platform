# Log Parser Test Creator Deliverables

## Purpose
Provide a repeatable test-creation workflow for CI logs and user-provided test logs.

## Must-Have Deliverables
1. Fixture package format under `tests/services/fixtures/<fixture_name>/`.
2. Required files per fixture:
   - `input.log` (raw log input)
   - `expected.json` (golden parser output)
   - `metadata.json` (source, language, framework, confidence band, notes)
3. Contract tests that validate fixture completeness and schema keys.
4. Golden-output tests that compare parser output against `expected.json`.
5. Determinism tests (same input produces same output).
6. Performance sanity test for large logs.
7. Fixture generator utility for onboarding new logs quickly.

## User-Provided Test Logs
User-provided tests are treated as first-class input. The source should be set to `user` in `metadata.json`.

## Creating A New Fixture
Run from `ai_engine` directory:

python tests/tools/create_log_fixture.py \
  --name "my failing test" \
  --source user \
  --language python \
  --framework pytest \
  --log-file /path/to/raw.log

Then:
1. Execute parser against `input.log`.
2. Copy the parser result into `expected.json`.
3. Set `expected_confidence_band` and notes.
4. Run tests.

## Acceptance Gate
A fixture is considered ready when:
1. All required files exist.
2. `expected.json` uses the parser schema keys.
3. Golden-output test passes.
4. The fixture has clear notes for future triage.
