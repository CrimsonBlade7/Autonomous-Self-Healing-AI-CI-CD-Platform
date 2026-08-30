from app.services.log_analyzer import parse_log


def test_empty_log_returns_canonical_unknown():
    result = parse_log("")
    assert result["error_type"] == "UnknownError"
    assert result["failing_test"] == "unknown"
    assert result["stack_trace_lines"] == []
    assert result["error_signature"] == "UnknownError | unknown | empty log"


def test_keeps_deterministic_output_for_same_input():
    raw_log = """
    Traceback (most recent call last):
      File \"test.py\", line 10, in <module>
        assert False, \"boom\"
    AssertionError: boom
    """
    first = parse_log(raw_log)
    second = parse_log(raw_log)
    assert first == second


def test_ansi_sequences_are_removed_before_parsing():
    raw_log = "\x1b[31mAssertionError: boom\x1b[0m"
    result = parse_log(raw_log)
    assert result["error_type"] == "AssertionError"
    assert result["error_signature"] == "AssertionError | unknown | boom"
