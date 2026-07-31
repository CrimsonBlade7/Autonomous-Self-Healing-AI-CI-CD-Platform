from app.services.log_parsers import parse_log


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


def test_rust_panic_log():
    log = """
    running 1 test
    test tests::it_fails ... FAILED

    failures:

    ---- tests::it_fails stdout ----
    thread 'tests::it_fails' panicked at src/lib.rs:10:5:
    assertion failed: 1 == 2
    note: run with `RUST_BACKTRACE=1` environment variable to display a backtrace
    """
    result = parse_log(log)
    assert result["error_type"] == "PanicError"
    assert result["failing_test"] == "tests::it_fails"
    assert result["stack_trace_lines"][0].startswith("thread 'tests::it_fails' panicked at")
    assert "assertion failed: 1 == 2" in result["error_signature"]


def test_rust_compile_error_log():
        log = """
        error[E0425]: cannot find value `foo` in this scope
            --> src/main.rs:2:5
        error: could not compile `demo` due to 1 previous error
        """
        result = parse_log(log)
        assert result["error_type"] == "RustCompileError"
        assert result["failing_test"] == "unknown"
        assert result["stack_trace_lines"][0] == "error[E0425]: cannot find value `foo` in this scope"
        assert result["error_signature"] == "RustCompileError | unknown | cannot find value `foo` in this scope"


def test_node_jest_type_error_log():
        log = """
        FAIL tests/sum.test.ts
            ● sum › should add two numbers

        TypeError: Cannot read properties of undefined (reading 'value')
            at Object.<anonymous> (tests/sum.test.ts:8:14)
        """
        result = parse_log(log)
        assert result["error_type"] == "TypeError"
        assert result["failing_test"] == "sum › should add two numbers"
        assert result["stack_trace_lines"][0] == "TypeError: Cannot read properties of undefined (reading 'value')"
        assert "Cannot read properties of undefined" in result["error_signature"]


def test_go_panic_log():
        log = """
        --- FAIL: TestDivideByZero (0.00s)
        panic: runtime error: integer divide by zero [recovered]
        goroutine 6 [running]:
        testing.tRunner(0x14000122000, 0x1042cf6f0)
        """
        result = parse_log(log)
        assert result["error_type"] == "GoPanicError"
        assert result["failing_test"] == "TestDivideByZero"
        assert result["stack_trace_lines"][0] == "--- FAIL: TestDivideByZero (0.00s)"
        assert "integer divide by zero" in result["error_signature"]


def test_java_caused_by_log():
        log = """
        [ERROR] Tests run: 12, Failures: 0, Errors: 1, Skipped: 0 - in com.example.PaymentServiceTest
        Exception in thread "main" java.lang.RuntimeException: startup failed
        Caused by: java.lang.IllegalArgumentException: missing config value
        """
        result = parse_log(log)
        assert result["error_type"] == "IllegalArgumentException"
        assert result["failing_test"] == "com.example.PaymentServiceTest"
        assert result["stack_trace_lines"][0].startswith("Exception in thread")
        assert result["error_signature"] == "IllegalArgumentException | com.example.PaymentServiceTest | missing config value"