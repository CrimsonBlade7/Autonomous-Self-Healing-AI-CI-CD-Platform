"""Tests for the RAG pipeline (Phase 3).

All tests are unit tests: the database session and embedding model are mocked
so no real PostgreSQL or sentence-transformers download is needed.
"""

from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import numpy as np
import pytest

from app.database.vector_store import SimilarResult
from app.services.log_analyzer import parse_log
from app.services.rag_pipeline import (
    _format_similar_results,
    build_context_window,
    embed_text,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_PYTHON_ERROR: dict = {
    "error_type": "AssertionError",
    "failing_test": "tests/test_math.py::test_divide",
    "stack_trace_lines": [
        "Traceback (most recent call last):",
        'File "tests/test_math.py", line 10, in test_divide',
        "assert divide(1, 0) == 0",
        "AssertionError: divide by zero guard missing",
    ],
    "error_signature": "AssertionError | tests/test_math.py::test_divide | divide by zero guard missing",
}

_STUB_VECTOR = [0.0] * 384


def _stub_session(results: list[SimilarResult] | None = None) -> AsyncMock:
    """Return an AsyncMock that quacks like an AsyncSession."""
    session = AsyncMock()
    return session


def _similar_results(n: int = 3) -> list[SimilarResult]:
    return [
        SimilarResult(
            id=i,
            source_type="error_log",
            content_text=f"Historical error {i}\nLine two of error {i}",
            distance=0.1 * i,
        )
        for i in range(1, n + 1)
    ]


# ---------------------------------------------------------------------------
# embed_text
# ---------------------------------------------------------------------------


def test_embed_text_returns_correct_length():
    """embed_text must return a 384-element list matching the model dimension."""
    fake_vector = np.random.rand(384).astype("float32")
    mock_model = MagicMock()
    mock_model.encode.return_value = fake_vector

    with patch("app.services.rag_pipeline._get_model", return_value=mock_model):
        result = embed_text("AssertionError | unknown | boom")

    assert isinstance(result, list)
    assert len(result) == 384


def test_embed_text_calls_normalize_embeddings():
    """embed_text must pass normalize_embeddings=True to the model."""
    mock_model = MagicMock()
    mock_model.encode.return_value = np.zeros(384, dtype="float32")

    with patch("app.services.rag_pipeline._get_model", return_value=mock_model):
        embed_text("some text")

    mock_model.encode.assert_called_once_with("some text", normalize_embeddings=True)


# ---------------------------------------------------------------------------
# _format_similar_results
# ---------------------------------------------------------------------------


def test_format_similar_results_empty():
    output = _format_similar_results([])
    assert "(no historical matches found)" in output


def test_format_similar_results_includes_all_entries():
    results = _similar_results(n=3)
    output = _format_similar_results(results)

    for i in range(1, 4):
        assert f"Historical error {i}" in output
        assert f"[{i}]" in output


def test_format_similar_results_shows_distance():
    results = [SimilarResult(id=1, source_type="patch", content_text="fix it", distance=0.2345)]
    output = _format_similar_results(results)
    assert "0.2345" in output


def test_format_similar_results_shows_source_type():
    results = [SimilarResult(id=1, source_type="source_file", content_text="code here", distance=0.1)]
    output = _format_similar_results(results)
    assert "source_file" in output


# ---------------------------------------------------------------------------
# build_context_window — section presence
# ---------------------------------------------------------------------------


@pytest.fixture
def mock_embed():
    with patch(
        "app.services.rag_pipeline.embed_text",
        return_value=_STUB_VECTOR,
    ) as m:
        yield m


@pytest.fixture
def mock_search_empty():
    with patch(
        "app.services.rag_pipeline.similarity_search",
        new_callable=AsyncMock,
        return_value=[],
    ) as m:
        yield m


@pytest.fixture
def mock_search_with_results():
    with patch(
        "app.services.rag_pipeline.similarity_search",
        new_callable=AsyncMock,
        return_value=_similar_results(n=3),
    ) as m:
        yield m


def test_context_window_contains_error_summary_section(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session)
    )
    assert "[Error Summary]" in window
    assert "AssertionError" in window
    assert "tests/test_math.py::test_divide" in window


def test_context_window_contains_stack_trace_section(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session)
    )
    assert "[Stack Trace]" in window
    assert "Traceback (most recent call last):" in window


def test_context_window_stack_trace_missing_shows_placeholder(mock_embed, mock_search_empty):
    error_no_trace = {**_PYTHON_ERROR, "stack_trace_lines": []}
    session = _stub_session()
    window = asyncio.run(
        build_context_window(error_no_trace, session)
    )
    assert "(no traceback captured)" in window


def test_context_window_contains_failing_code_section(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session, failing_code="def divide(a, b): return a / b")
    )
    assert "[Failing Code Snippet]" in window
    assert "def divide(a, b):" in window


def test_context_window_no_failing_code_shows_placeholder(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session, failing_code="")
    )
    assert "(no source file provided)" in window


def test_context_window_contains_historical_section(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session)
    )
    assert "[Top Similar Historical Incidents]" in window


def test_context_window_includes_retrieved_matches(mock_embed, mock_search_with_results):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session)
    )
    assert "Historical error 1" in window
    assert "Historical error 3" in window


def test_context_window_no_matches_shows_placeholder(mock_embed, mock_search_empty):
    session = _stub_session()
    window = asyncio.run(
        build_context_window(_PYTHON_ERROR, session)
    )
    assert "(no historical matches found)" in window


# ---------------------------------------------------------------------------
# build_context_window — embedding integration
# ---------------------------------------------------------------------------


def test_context_window_embeds_error_signature(mock_search_empty):
    """embed_text must be called with the error_signature, not the raw type."""
    mock_model = MagicMock()
    mock_model.encode.return_value = np.zeros(384, dtype="float32")

    with patch("app.services.rag_pipeline._get_model", return_value=mock_model):
        session = _stub_session()
        asyncio.run(
            build_context_window(_PYTHON_ERROR, session)
        )

    mock_model.encode.assert_called_once_with(
        _PYTHON_ERROR["error_signature"],
        normalize_embeddings=True,
    )


def test_context_window_passes_top_k_to_similarity_search(mock_embed):
    with patch(
        "app.services.rag_pipeline.similarity_search",
        new_callable=AsyncMock,
        return_value=[],
    ) as mock_search:
        session = _stub_session()
        asyncio.run(
            build_context_window(_PYTHON_ERROR, session, top_k=7)
        )
        mock_search.assert_awaited_once()
        _, kwargs = mock_search.call_args
        assert kwargs.get("top_k") == 7 or mock_search.call_args[0][2] == 7


# ---------------------------------------------------------------------------
# build_context_window — round-trip with parse_log
# ---------------------------------------------------------------------------


def test_context_window_round_trip_from_parse_log(mock_embed, mock_search_empty):
    """parse_log output feeds directly into build_context_window without errors."""
    raw_log = (
        "Traceback (most recent call last):\n"
        '  File "app/math.py", line 5, in test_add\n'
        "    assert add(1, 2) == 4\n"
        "AssertionError: assert 3 == 4\n"
    )
    parsed = parse_log(raw_log)
    session = _stub_session()

    window = asyncio.run(
        build_context_window(parsed, session)
    )

    assert "[Error Summary]" in window
    assert parsed["error_type"] in window
    assert parsed["error_signature"] in window
