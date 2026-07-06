from ai_engine.app.services.log_analyzer import parse_log


def test_empty_log():
    result = parse_log("")
    assert result["error_type"] == "UnknownError"
    assert result["failing_test"] == "unknown"
    assert result["stack_trace_lines"] == []
    assert result["error_signature"] == "UnknownError | unknown | empty log"


def test_noisy_log():
    log = """
    This is a log with no errors.
    Everything is fine.
    """
    result = parse_log(log)
    assert result["error_type"] == "UnknownError"
    assert result["failing_test"] == "unknown"
    assert result["stack_trace_lines"] == ["This is a log with no errors."]
    assert result["error_signature"] == "UnknownError | unknown | no message"


def test_unknown_format_log():
    log = """
    Some random log content that doesn't match any known format.
    """
    result = parse_log(log)
    assert result["error_type"] == "UnknownError"
    assert result["failing_test"] == "unknown"
    assert result["stack_trace_lines"] == []
    assert result["error_signature"] == "UnknownError | unknown | no message"


def test_python_log():
    log = """
    Traceback (most recent call last):
      File "test.py", line 10, in <module>
        assert False, "This is a test failure"
    AssertionError: This is a test failure
    """
    result = parse_log(log)
    assert result["error_type"] == "AssertionError"
    assert result["failing_test"] == "unknown"
    assert result["stack_trace_lines"] == [
        "Traceback (most recent call last):",
        'File "test.py", line 10, in <module>',
        'assert False, "This is a test failure"',
        "AssertionError: This is a test failure",
    ]
    assert result["error_signature"] == "AssertionError | unknown | This is a test failure"