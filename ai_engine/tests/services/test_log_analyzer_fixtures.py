import json
from pathlib import Path

from app.services.log_analyzer import parse_log


FIXTURE_ROOT = Path(__file__).parent / "fixtures"


def _load_json(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def _fixture_dirs() -> list[Path]:
    return sorted([p for p in FIXTURE_ROOT.iterdir() if p.is_dir()])


def test_fixture_contract_files_present():
    for fixture_dir in _fixture_dirs():
        assert (fixture_dir / "input.log").exists()
        assert (fixture_dir / "expected.json").exists()
        assert (fixture_dir / "metadata.json").exists()


def test_fixture_expected_schema_keys_present():
    required = {
        "error_type",
        "failing_test",
        "stack_trace_lines",
        "error_signature",
    }
    for fixture_dir in _fixture_dirs():
        expected = _load_json(fixture_dir / "expected.json")
        assert required.issubset(set(expected.keys()))


def test_fixture_metadata_keys_present():
    required = {
        "name",
        "source",
        "language",
        "framework",
        "expected_confidence_band",
        "notes",
    }
    for fixture_dir in _fixture_dirs():
        metadata = _load_json(fixture_dir / "metadata.json")
        assert required.issubset(set(metadata.keys()))


def test_parse_log_matches_expected_output():
    for fixture_dir in _fixture_dirs():
        raw_log = (fixture_dir / "input.log").read_text(encoding="utf-8")
        expected = _load_json(fixture_dir / "expected.json")

        parsed = parse_log(raw_log)

        assert parsed["error_type"] == expected["error_type"]
        assert parsed["failing_test"] == expected["failing_test"]
        assert parsed["stack_trace_lines"] == expected["stack_trace_lines"]
        assert parsed["error_signature"] == expected["error_signature"]
