# Phase 3: RAG Pipeline & Tests — Summary

## What Was Built

### `app/database/vector_store.py`
Implements the persistence and retrieval layer backed by PostgreSQL + pgvector.

| Symbol | Description |
|---|---|
| `CodeEmbedding` | SQLAlchemy ORM model for the `code_embeddings` table (migration 000002) |
| `SimilarResult` | Dataclass returned from similarity search: `id`, `source_type`, `content_text`, `distance` |
| `save_embedding(session, content, source_type, embedding, pipeline_run_id)` | Persists a text snippet and its 384-dim vector; flushes the row and returns it |
| `similarity_search(session, query_vector, top_k=5)` | Cosine distance search via pgvector `<=>` operator, accelerated by the HNSW index |

### `app/services/rag_pipeline.py`
Orchestrates embedding → retrieval → context assembly for the LLM (Phase 4 input).

| Symbol | Description |
|---|---|
| `embed_text(text)` | Encodes text with `all-MiniLM-L6-v2`; model is lazy-loaded once via `lru_cache` |
| `build_context_window(parsed_error, session, failing_code, top_k)` | Async function that produces the four-section context window below |

**Context window sections:**
```
[Error Summary]       — error_type, failing_test, error_signature
[Stack Trace]         — stack_trace_lines or "(no traceback captured)"
[Failing Code Snippet]— failing_code content or "(no source file provided)"
[Top Similar Historical Incidents] — top_k nearest neighbours from vector store
```

### `tests/services/test_rag_pipeline.py`
17 unit tests. All external I/O (DB session, embedding model) is mocked.

| Test group | What is covered |
|---|---|
| `embed_text` | Returns 384-element list; passes `normalize_embeddings=True` |
| `_format_similar_results` | Empty list placeholder; all entries rendered; distance/source type shown |
| `build_context_window` — sections | All four section headers present; placeholder text when data missing |
| `build_context_window` — content | Retrieved match content appears; failing code snippet injected |
| `build_context_window` — integration | `top_k` forwarded to similarity search; signature (not raw type) is embedded |
| Round-trip | `parse_log` output feeds directly into `build_context_window` without errors |

## Data Flow

```
raw_log
  └─► log_analyzer.parse_log()        → ParsedError
        └─► rag_pipeline.embed_text()  → 384-dim vector
              └─► vector_store.similarity_search()  → List[SimilarResult]
                    └─► rag_pipeline.build_context_window()  → context_window: str
                          └─► patch_gen (Phase 4)
```

## Key Design Decisions

- **Lazy model loading** — `SentenceTransformer` is instantiated once at first call, not at import time, avoiding slow startup during testing and task-worker boot.
- **Cosine distance** — The `<=>` pgvector operator matches the `vector_cosine_ops` HNSW index in migration 000002, keeping retrieval fast at scale.
- **`asyncio.run()` in tests** — All async test coroutines use `asyncio.run()` to avoid the deprecated `get_event_loop()` pattern.
- **`session.flush()` in `save_embedding`** — Assigns a DB-generated `id` without ending the caller's transaction, keeping the save composable with surrounding Celery tasks.
