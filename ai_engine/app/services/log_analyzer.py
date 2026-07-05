# Phase 2: Log Analyzer
#
# Parses raw Docker STDOUT/STDERR logs produced by a failing CI job into a
# structured error object that downstream services (RAG pipeline, patch generator)
# can consume.
#
# Input:  raw_log: str  — the full text captured from the container's output.
#
# Output: dict with the following keys:
#   {
#       "error_type":        str,   # e.g. "AssertionError", "ImportError"
#       "failing_test":      str,   # fully-qualified test name, if found
#       "stack_trace_lines": list[str],  # relevant stack trace lines
#       "error_signature":   str,   # compact single-line summary for embedding
#   }
#
# The error_signature is fed into the RAG pipeline (Phase 3) as the query vector.
#
# Implementation arrives in Phase 2.
