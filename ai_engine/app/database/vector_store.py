"""Vector store: persist and retrieve embeddings via pgvector."""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np
from pgvector.sqlalchemy import Vector
from sqlalchemy import Integer, Text, select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import Mapped, mapped_column

from app.core.config import settings
from app.database.session import Base


class CodeEmbedding(Base):
    """ORM model that mirrors the code_embeddings table (migration 000002)."""

    __tablename__ = "code_embeddings"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    pipeline_run_id: Mapped[int | None] = mapped_column(Integer, nullable=True)
    # 'error_log' | 'source_file' | 'patch'
    source_type: Mapped[str] = mapped_column(Text, nullable=False)
    content_text: Mapped[str] = mapped_column(Text, nullable=False)
    embedding: Mapped[list[float]] = mapped_column(
        Vector(settings.embedding_dimensions), nullable=True
    )


@dataclass
class SimilarResult:
    """A retrieved embedding row together with its cosine distance."""

    id: int
    source_type: str
    content_text: str
    distance: float


async def save_embedding(
    session: AsyncSession,
    content: str,
    source_type: str,
    embedding: list[float] | np.ndarray,
    pipeline_run_id: int | None = None,
) -> CodeEmbedding:
    """Persist a text snippet and its vector to code_embeddings."""
    vec = embedding.tolist() if isinstance(embedding, np.ndarray) else list(embedding)
    row = CodeEmbedding(
        pipeline_run_id=pipeline_run_id,
        source_type=source_type,
        content_text=content,
        embedding=vec,
    )
    session.add(row)
    await session.flush()  # assigns .id without ending the transaction
    return row


async def similarity_search(
    session: AsyncSession,
    query_vector: list[float] | np.ndarray,
    top_k: int = 5,
) -> list[SimilarResult]:
    """Return the top_k rows most similar to query_vector (cosine distance)."""
    vec = query_vector.tolist() if isinstance(query_vector, np.ndarray) else list(query_vector)

    # cosine_distance() uses the <=> pgvector operator, which is accelerated
    # by the HNSW index created in migration 000002.
    stmt = (
        select(
            CodeEmbedding.id,
            CodeEmbedding.source_type,
            CodeEmbedding.content_text,
            CodeEmbedding.embedding.cosine_distance(vec).label("distance"),
        )
        .order_by("distance")
        .limit(top_k)
    )

    rows = (await session.execute(stmt)).all()
    return [
        SimilarResult(
            id=row.id,
            source_type=row.source_type,
            content_text=row.content_text,
            distance=float(row.distance),
        )
        for row in rows
    ]
