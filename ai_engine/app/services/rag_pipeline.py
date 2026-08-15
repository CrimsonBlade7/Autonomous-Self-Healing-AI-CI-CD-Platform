"""Phase 3: RAG Pipeline.

Embeds a parsed error signature, retrieves historically similar incidents from
the vector store, and assembles a structured context window for the LLM.
"""

from __future__ import annotations

import functools

from sentence_transformers import SentenceTransformer
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import settings
from app.database.vector_store import SimilarResult, similarity_search
from app.services.log_analyzer import ParsedError


@functools.lru_cache(maxsize=1)
def _get_model() -> SentenceTransformer:
    """Load the embedding model once and reuse it for the process lifetime."""
    return SentenceTransformer(settings.embedding_model)


def embed_text(text: str) -> list[float]:
    """Return a 384-dimensional embedding vector for *text*."""
    model = _get_model()
    vector = model.encode(text, normalize_embeddings=True)
    return vector.tolist()


def _format_similar_results(results: list[SimilarResult]) -> str:
    if not results:
        return "  (no historical matches found)"
    lines: list[str] = []
    for i, r in enumerate(results, start=1):
        lines.append(f"  [{i}] type={r.source_type} distance={r.distance:.4f}")
        # indent the content so it reads cleanly inside the context window
        for content_line in r.content_text.splitlines():
            lines.append(f"      {content_line}")
    return "\n".join(lines)


async def build_context_window(
    parsed_error: ParsedError,
    session: AsyncSession,
    failing_code: str = "",
    top_k: int = 5,
) -> str:
    """Assemble the LLM context window from a parsed failure + historical store.

    Args:
        parsed_error: Structured output from log_analyzer.parse_log().
        session:      Active async DB session (injected by FastAPI / Celery task).
        failing_code: Source code of the failing file, if available.
        top_k:        Number of similar historical records to retrieve.

    Returns:
        A multi-section string ready to be embedded in an LLM prompt.
    """
    query_vector = embed_text(parsed_error["error_signature"])
    similar_results = await similarity_search(session, query_vector, top_k=top_k)

    sections: list[str] = []

    # -- Error Summary --------------------------------------------------------
    sections.append("[Error Summary]")
    sections.append(f"  Type:      {parsed_error['error_type']}")
    sections.append(f"  Test:      {parsed_error['failing_test']}")
    sections.append(f"  Signature: {parsed_error['error_signature']}")

    # -- Stack Trace ----------------------------------------------------------
    sections.append("\n[Stack Trace]")
    if parsed_error["stack_trace_lines"]:
        for line in parsed_error["stack_trace_lines"]:
            sections.append(f"  {line}")
    else:
        sections.append("  (no traceback captured)")

    # -- Failing Code Snippet -------------------------------------------------
    sections.append("\n[Failing Code Snippet]")
    if failing_code.strip():
        for line in failing_code.splitlines():
            sections.append(f"  {line}")
    else:
        sections.append("  (no source file provided)")

    # -- Historical Similar Incidents -----------------------------------------
    sections.append("\n[Top Similar Historical Incidents]")
    sections.append(_format_similar_results(similar_results))

    return "\n".join(sections)
