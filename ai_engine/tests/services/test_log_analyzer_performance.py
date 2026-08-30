import time

from app.services.log_analyzer import parse_log


def _make_large_log(size_mb: int) -> str:
    line = "INFO build step completed successfully\n"
    repeated = (size_mb * 1024 * 1024) // len(line)
    log_blob = line * repeated
    return log_blob + "AssertionError: synthetic failure at tail\n"


def test_parse_log_large_input_completes_under_budget():
    raw_log = _make_large_log(size_mb=5)

    start = time.perf_counter()
    parsed = parse_log(raw_log)
    elapsed = time.perf_counter() - start

    assert parsed["error_type"] in {"AssertionError", "UnknownError"}
    assert elapsed < 3.0
